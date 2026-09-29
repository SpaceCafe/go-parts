package config

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidDuration is returned for a JSON string in a time.Duration field that
// time.ParseDuration rejects.
var ErrInvalidDuration = errors.New("config: invalid duration")

//nolint:gochecknoglobals // Read-only reflection types.
var (
	durationType         = reflect.TypeFor[time.Duration]()
	jsonUnmarshalerType  = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType  = reflect.TypeFor[encoding.TextUnmarshaler]()
	errTrailingJSONValue = errors.New("trailing JSON value")
)

// durationConverter converts the JSON value of a time.Duration field.
type durationConverter func(value any) (any, error)

// jsonField is a struct field as encoding/json sees it: its key and its type.
type jsonField struct {
	typ  reflect.Type
	name string
}

// durationStringToNanos converts a duration string such as "1m30s" into integer nanoseconds, the
// only form encoding/json reads into a time.Duration. Other values are left for the decoder.
func durationStringToNanos(value any) (any, error) {
	text, ok := value.(string)
	if !ok {
		return value, nil
	}

	duration, err := time.ParseDuration(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDuration, err)
	}

	return json.Number(strconv.FormatInt(int64(duration), 10)), nil
}

// durationNanosToString converts integer nanoseconds into a duration string such as "1m30s".
func durationNanosToString(value any) (any, error) {
	number, ok := value.(json.Number)
	if !ok {
		return value, nil
	}

	nanos, err := number.Int64()
	if err != nil {
		return value, nil //nolint:nilerr // Not an integer: leave it for the decoder to report.
	}

	return time.Duration(nanos).String(), nil
}

// rewriteJSONDurations applies convert to every value in data that target decodes into a
// time.Duration field. Data that is not a single JSON value is returned unchanged, so the decoder
// reports the syntax error or trailing data as before.
func rewriteJSONDurations(data []byte, target any, convert durationConverter) ([]byte, error) {
	tree, err := decodeJSONTree(data)
	if err != nil {
		return data, nil //nolint:nilerr // The caller's decoder reports the error.
	}

	tree, err = walkJSONDurations(tree, reflect.TypeOf(target), convert, "")
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal JSON: %w", ErrInvalidConfig, err)
	}

	return result, nil
}

// decodeJSONTree decodes data into generic values, keeping numbers as json.Number so large
// integers keep their precision. It fails for anything but a single JSON value.
func decodeJSONTree(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var tree any

	err := decoder.Decode(&tree)
	if err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	_, err = decoder.Token()
	if !errors.Is(err, io.EOF) {
		return nil, errTrailingJSONValue
	}

	return tree, nil
}

// walkJSONDurations follows typ through the generic JSON value and applies convert to the values
// of time.Duration fields. Types with their own JSON or text unmarshaling are left alone. path
// names the current position for error messages.
func walkJSONDurations(
	value any,
	typ reflect.Type,
	convert durationConverter,
	path string,
) (any, error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ == durationType {
		converted, err := convert(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalidConfig, path, err)
		}

		return converted, nil
	}

	pointer := reflect.PointerTo(typ)
	if pointer.Implements(jsonUnmarshalerType) || pointer.Implements(textUnmarshalerType) {
		return value, nil
	}

	//nolint:exhaustive // Only these kinds can contain a time.Duration; every other kind is a leaf.
	switch typ.Kind() {
	case reflect.Struct:
		return walkJSONObject(value, typ, convert, path)
	case reflect.Map:
		return walkJSONMap(value, typ.Elem(), convert, path)
	case reflect.Slice, reflect.Array:
		return walkJSONArray(value, typ.Elem(), convert, path)
	default:
		return value, nil
	}
}

// walkJSONObject walks the members of a JSON object that decodes into the struct type typ.
func walkJSONObject(
	value any,
	typ reflect.Type,
	convert durationConverter,
	path string,
) (any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	fields := jsonFields(typ)

	for key, item := range object {
		field, found := lookupJSONField(fields, key)
		if !found {
			continue
		}

		converted, err := walkJSONDurations(item, field.typ, convert, joinJSONPath(path, key))
		if err != nil {
			return nil, err
		}

		object[key] = converted
	}

	return object, nil
}

// walkJSONMap walks the values of a JSON object that decodes into a map with element type elem.
func walkJSONMap(
	value any,
	elem reflect.Type,
	convert durationConverter,
	path string,
) (any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	for key, item := range object {
		converted, err := walkJSONDurations(item, elem, convert, joinJSONPath(path, key))
		if err != nil {
			return nil, err
		}

		object[key] = converted
	}

	return object, nil
}

// walkJSONArray walks the elements of a JSON array that decodes into a slice or array of elem.
func walkJSONArray(
	value any,
	elem reflect.Type,
	convert durationConverter,
	path string,
) (any, error) {
	items, ok := value.([]any)
	if !ok {
		return value, nil
	}

	for index, item := range items {
		converted, err := walkJSONDurations(
			item,
			elem,
			convert,
			path+"["+strconv.Itoa(index)+"]",
		)
		if err != nil {
			return nil, err
		}

		items[index] = converted
	}

	return items, nil
}

// jsonFields lists the fields encoding/json decodes into for the struct type typ, including the
// fields promoted from embedded structs without a JSON name. Direct fields come first, so they win
// over promoted ones, as in encoding/json.
func jsonFields(typ reflect.Type) []jsonField {
	var (
		fields   []jsonField
		promoted []jsonField
	)

	for field := range typ.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}

		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if field.Anonymous && name == "" && fieldType.Kind() == reflect.Struct {
			promoted = append(promoted, jsonFields(fieldType)...)

			continue
		}

		if !field.IsExported() {
			continue
		}

		if name == "" {
			name = field.Name
		}

		fields = append(fields, jsonField{name: name, typ: field.Type})
	}

	return append(fields, promoted...)
}

// lookupJSONField finds the field for key like encoding/json: an exact match first, then a
// case-insensitive one.
func lookupJSONField(fields []jsonField, key string) (jsonField, bool) {
	for _, field := range fields {
		if field.name == key {
			return field, true
		}
	}

	for _, field := range fields {
		if strings.EqualFold(field.name, key) {
			return field, true
		}
	}

	return jsonField{}, false
}

// joinJSONPath appends key to path with a dot.
func joinJSONPath(path, key string) string {
	if path == "" {
		return key
	}

	return path + "." + key
}
