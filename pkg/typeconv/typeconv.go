// Package typeconv provides utilities for converting string values to various Go types
// using reflection. This is useful for configuration loading, CLI parsing, and other
// scenarios where string data needs to be converted to strongly typed values.
package typeconv

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnsupportedType = errors.New("typeconv: unsupported type")
	ErrInvalidValue    = errors.New("typeconv: invalid value")
)

// Converter handles conversion of string values to various Go types.
type Converter struct {
	// SliceSeparator is the string used to split slice values. Default is ",".
	SliceSeparator string

	// MapKeyValueSeparator is the string used to separate keys from values in map entries. Default is "=".
	MapKeyValueSeparator string

	// TimeLayout is the layout used for time.Time conversion. Default is time.RFC3339.
	TimeLayout string
}

// New creates a new Converter with default settings.
func New() *Converter {
	return &Converter{
		SliceSeparator:       ",",
		MapKeyValueSeparator: "=",
		TimeLayout:           time.RFC3339,
	}
}

// Default is the default converter instance.
//
//nolint:gochecknoglobals // Default converter instance provided for convenience
var Default = New()

// Convert converts a string value to the type of the target reflect.Value.
// The target must be a settable (e.g., from reflect.ValueOf(&x).Elem()).

func (c *Converter) Convert(target reflect.Value, value string) error {
	if !target.CanSet() {
		return fmt.Errorf("%w: target value is not settable", ErrUnsupportedType)
	}

	return c.setField(target, value)
}

// ConvertTo converts a string value to the specified type T.
// This is a generic helper that returns the converted value.
//
//nolint:ireturn // Generic function must return type parameter T.
func ConvertTo[T any](value string) (T, error) {
	var result T

	v := reflect.ValueOf(&result).Elem()

	err := Default.Convert(v, value)
	if err != nil {
		return result, err
	}

	return result, nil
}

// MustConvertTo is like ConvertTo but panics on error.
//
//nolint:ireturn // Generic function must return type parameter T.
func MustConvertTo[T any](value string) T {
	result, err := ConvertTo[T](value)
	if err != nil {
		panic(err)
	}

	return result
}

// setField sets the field value from the string.
func (c *Converter) setField(field reflect.Value, value string) error {
	// Check if the type implements encoding.TextUnmarshaler
	if field.CanAddr() {
		if unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			err := unmarshaler.UnmarshalText([]byte(value))
			if err != nil {
				return fmt.Errorf("%w: failed to unmarshal: %w", ErrInvalidValue, err)
			}

			return nil
		}
	}

	if field.Type() == reflect.TypeFor[time.Duration]() {
		return setDuration(field, value)
	}

	if field.Type() == reflect.TypeFor[time.Time]() {
		return setTime(field, value, c.TimeLayout)
	}

	//nolint:exhaustive // Only handling supported reflect.Kind types; unsupported types handled by default case.
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setInt(field, value)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUint(field, value)

	case reflect.Float32, reflect.Float64:
		return setFloat(field, value)

	case reflect.Bool:
		return setBool(field, value)

	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}

		return c.setField(field.Elem(), value)

	case reflect.Slice:
		return c.setSlice(field, value)

	case reflect.Map:
		return c.setMap(field, value)

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, field.Kind())
	}

	return nil
}

// setMap handles map conversion by parsing "key=value" pairs separated by unquoted whitespace.
// Double-quoted values are supported to allow spaces within values (or keys).
func (c *Converter) setMap(field reflect.Value, value string) error {
	if value == "" {
		field.Set(reflect.MakeMap(field.Type()))

		return nil
	}

	entries, err := parseMapEntries(value, c.MapKeyValueSeparator)
	if err != nil {
		return err
	}

	newMap := reflect.MakeMapWithSize(field.Type(), len(entries))

	for _, entry := range entries {
		mapKey := reflect.New(field.Type().Key()).Elem()

		err = c.setField(mapKey, entry[0])
		if err != nil {
			return fmt.Errorf("typeconv: map key '%s': %w", entry[0], err)
		}

		mapValue := reflect.New(field.Type().Elem()).Elem()

		err = c.setField(mapValue, entry[1])
		if err != nil {
			return fmt.Errorf("typeconv: map value '%s': %w", entry[1], err)
		}

		newMap.SetMapIndex(mapKey, mapValue)
	}

	field.Set(newMap)

	return nil
}

// setSlice handles slice conversion by splitting the value and converting each element.
func (c *Converter) setSlice(field reflect.Value, value string) error {
	if value == "" {
		// Empty string creates an empty slice.
		field.Set(reflect.MakeSlice(field.Type(), 0, 0))

		return nil
	}

	parts := strings.Split(value, c.SliceSeparator)
	slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))

	for i, part := range parts {
		part = strings.TrimSpace(part)
		elem := slice.Index(i)

		// For pointer element types, create a new instance.
		if elem.Kind() == reflect.Ptr {
			elem.Set(reflect.New(elem.Type().Elem()))
			elem = elem.Elem()
		}

		err := c.setField(elem, part)
		if err != nil {
			return fmt.Errorf("typeconv: slice element '%d': %w", i, err)
		}
	}

	field.Set(slice)

	return nil
}

// parseMapEntries parses a string of key-value pairs separated by unquoted whitespace.
// Each pair is split on the first occurrence of the kvSep separator.
// Double-quoted segments are respected, so spaces inside quotes are preserved.
// Returns a slice of [2]string{key, value} entries.
func parseMapEntries(input, kvSep string) ([][2]string, error) {
	tokens := splitUnquoted(input)
	entries := make([][2]string, 0, len(tokens))

	for _, token := range tokens {
		key, val, found := strings.Cut(token, kvSep)
		if !found {
			return nil, fmt.Errorf(
				"%w: map entry '%s' missing '%s' separator",
				ErrInvalidValue,
				token,
				kvSep,
			)
		}

		entries = append(entries, [2]string{unquote(key), unquote(val)})
	}

	return entries, nil
}

func setBool(field reflect.Value, value string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "1", "t", "y", "true", "yes", "on":
		field.SetBool(true)

		return nil
	case "0", "f", "n", "false", "no", "off":
		field.SetBool(false)

		return nil
	}

	return fmt.Errorf("%w: cannot parse '%s' as bool", ErrInvalidValue, value)
}

func setFloat(field reflect.Value, value string) error {
	floatVal, err := strconv.ParseFloat(value, field.Type().Bits())
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as float: %w", ErrInvalidValue, value, err)
	}

	if field.OverflowFloat(floatVal) {
		return fmt.Errorf("%w: value '%f' overflows '%s'", ErrInvalidValue, floatVal, field.Type())
	}

	field.SetFloat(floatVal)

	return nil
}

func setInt(field reflect.Value, value string) error {
	intVal, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as int: %w", ErrInvalidValue, value, err)
	}

	if field.OverflowInt(intVal) {
		return fmt.Errorf("%w: value '%d' overflows '%s'", ErrInvalidValue, intVal, field.Type())
	}

	field.SetInt(intVal)

	return nil
}

func setUint(field reflect.Value, value string) error {
	uintVal, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as uint: %w", ErrInvalidValue, value, err)
	}

	if field.OverflowUint(uintVal) {
		return fmt.Errorf("%w: value '%d' overflows '%s'", ErrInvalidValue, uintVal, field.Type())
	}

	field.SetUint(uintVal)

	return nil
}

func setDuration(field reflect.Value, value string) error {
	durationVal, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as duration: %w", ErrInvalidValue, value, err)
	}

	field.SetInt(int64(durationVal))

	return nil
}

func setTime(field reflect.Value, value, layout string) error {
	timeVal, err := time.Parse(layout, value)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as time: %w", ErrInvalidValue, value, err)
	}

	field.Set(reflect.ValueOf(timeVal))

	return nil
}

// splitUnquoted splits a string on unquoted whitespace.
// Characters inside double quotes are treated as part of the current token.
func splitUnquoted(value string) []string {
	var (
		tokens []string
		token  strings.Builder
		inQuot bool
	)

	for i := range len(value) {
		char := value[i]

		switch {
		case char == '"':
			inQuot = !inQuot

			token.WriteByte(char)
		case char == ' ' && !inQuot:
			if token.Len() > 0 {
				tokens = append(tokens, token.String())
				token.Reset()
			}
		default:
			token.WriteByte(char)
		}
	}

	if token.Len() > 0 {
		tokens = append(tokens, token.String())
	}

	return tokens
}

// unquote removes surrounding double quotes from a string if present.
func unquote(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}

	return value
}
