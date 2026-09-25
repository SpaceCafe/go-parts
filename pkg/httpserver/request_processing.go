package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/spacecafe/go-parts/pkg/validate"
)

var (
	ErrJSONBodyDecoding = errors.New("httpserver: failed to decode JSON body")
	ErrNoKey            = errors.New("httpserver: key must not be empty")
	ErrRequestTooLarge  = errors.New("httpserver: request body too large")
)

// wrapBodyError maps the error returned by http.MaxBytesReader to ErrRequestTooLarge and wraps
// every other error with fallback.
func wrapBodyError(fallback, err error) error {
	if maxBytesErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return fmt.Errorf("%w: limit is %d bytes", ErrRequestTooLarge, maxBytesErr.Limit)
	}

	return fmt.Errorf("%w: %s", fallback, err.Error())
}

// GetFormValue retrieves and converts a form value from an HTTP request to the specified type. If
// the form value is missing, the defaultValue is returned. The validators are optional and perform
// additional validation on the value.
func GetFormValue[T any](
	req *http.Request,
	target *T,
	key string,
	defaultValue T,
	validators ...func(T) error,
) error {
	if key == "" {
		return ErrNoKey
	}

	return getFormValue[T](target, key, req.FormValue(key), defaultValue, validators...)
}

// GetJSONBody decodes the JSON-encoded body of an HTTP request into `v`. Unknown fields and data
// after the JSON value are rejected. If `v` implements a Validate error method, it is called after
// successful decoding and any returned error is propagated to the caller. On any error, the
// temporary files of File and Base64File fields in `v` are removed, because the caller does not
// get a usable result to clean up. The body is read without a limit of its own; wrap it with
// middleware.MaxBodySize and check for ErrRequestTooLarge to answer with 413.
func GetJSONBody(req *http.Request, target any) (err error) {
	defer func() {
		if err != nil {
			cleanupFiles(reflect.ValueOf(target), make(map[uintptr]struct{}))
		}
	}()

	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()

	err = dec.Decode(target)
	if err != nil {
		return wrapBodyError(ErrJSONBodyDecoding, err)
	}

	// A second value, or anything but whitespace, after the first one is trailing data.
	_, err = dec.Token()
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: unexpected data after JSON value", ErrJSONBodyDecoding)
		}

		return wrapBodyError(ErrJSONBodyDecoding, err)
	}

	type validator interface {
		Validate() error
	}

	if val, ok := target.(validator); ok {
		err = val.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// fileType is the type cleanupFiles looks for. Base64File is covered through its embedded File.
//
//nolint:gochecknoglobals // Constant reflect.Type, computed once.
var fileType = reflect.TypeFor[File]()

// cleanupFiles walks value and calls Cleanup on every File it reaches through exported struct
// fields, pointers, interfaces, slices, arrays and maps. seen guards against pointer cycles.
func cleanupFiles(value reflect.Value, seen map[uintptr]struct{}) {
	//nolint:exhaustive // Only these kinds can contain a File; every other kind is a leaf.
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return
		}

		if _, ok := seen[value.Pointer()]; ok {
			return
		}

		seen[value.Pointer()] = struct{}{}

		cleanupFiles(value.Elem(), seen)
	case reflect.Interface:
		if !value.IsNil() {
			cleanupFiles(value.Elem(), seen)
		}
	case reflect.Struct:
		if value.Type() == fileType {
			if file, ok := reflect.TypeAssert[File](value); ok && file.Cleanup != nil {
				_ = file.Cleanup()
			}

			return
		}

		for i := range value.NumField() {
			if value.Type().Field(i).IsExported() {
				cleanupFiles(value.Field(i), seen)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			cleanupFiles(value.Index(i), seen)
		}
	case reflect.Map:
		for iter := value.MapRange(); iter.Next(); {
			cleanupFiles(iter.Value(), seen)
		}
	}
}

// GetPathValue retrieves and converts a path value from an HTTP request to the specified
// type. If the path value is missing, the defaultValue is returned. The validators are optional
// and perform additional validation on the value.
func GetPathValue[T any](
	req *http.Request,
	target *T,
	key string,
	defaultValue T,
	validators ...func(T) error,
) error {
	if key == "" {
		return ErrNoKey
	}

	return getFormValue[T](target, key, req.PathValue(key), defaultValue, validators...)
}

// GetQueryParam retrieves and converts a query parameter from an HTTP request to the specified
// type. If the query parameter is missing, the defaultValue is returned. The validators are optional
// and perform additional validation on the value.
func GetQueryParam[T any](
	req *http.Request,
	target *T,
	key string,
	defaultValue T,
	validators ...func(T) error,
) error {
	if key == "" {
		return ErrNoKey
	}

	return getFormValue[T](target, key, req.URL.Query().Get(key), defaultValue, validators...)
}

// getFormValue retrieves and converts a given value to the specified type T, with optional
// validation. The key names the value in any error returned.
func getFormValue[T any](
	target *T,
	key string,
	formValue string,
	defaultValue T,
	validators ...func(T) error,
) error {
	*target = defaultValue

	if formValue == "" {
		return nil
	}

	value, err := typeconv.ConvertTo[T](formValue)
	if err != nil {
		// Report the raw form value: the converted value is the zero value here, not the input
		// the caller needs to see.
		return &validate.ValidationError{Name: key, Value: formValue, Err: err}
	}

	err = validate.Validate(key, value, validators...)
	if err != nil {
		return err
	}

	*target = value

	return nil
}
