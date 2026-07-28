package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/spacecafe/go-parts/pkg/validate"
)

var (
	ErrJSONBodyDecoding = errors.New("httpserver: failed to decode JSON body")
	ErrNoKey            = errors.New("httpserver: key must not be empty")
)

// GetFormValue retrieves and converts a form value from an HTTP request to the specified type. If
// the form value is missing, the defaultValue is returned. The validators are optional and perform
// additional validation on the value.
//
//nolint:ireturn // Generic function must return type parameter T.
func GetFormValue[T any](
	req *http.Request,
	key string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if key == "" {
		return defaultValue, nil
	}

	return getFormValue[T](key, req.FormValue(key), defaultValue, validators...)
}

// GetJSONBody decodes the JSON-encoded body of an HTTP request into `v`. If `v` implements a
// Validate error method, it is called after successful decoding and any
// returned error is propagated to the caller.
func GetJSONBody(req *http.Request, target any) error {
	err := json.NewDecoder(req.Body).Decode(target)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrJSONBodyDecoding, err.Error())
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

// GetPathValue retrieves and converts a path value from an HTTP request to the specified
// type. If the path value is missing, the defaultValue is returned. The validators are optional
// and perform additional validation on the value.
//
//nolint:ireturn // Generic function must return type parameter T.
func GetPathValue[T any](
	req *http.Request,
	key string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if key == "" {
		return defaultValue, ErrNoKey
	}

	return getFormValue[T](key, req.PathValue(key), defaultValue, validators...)
}

// GetQueryParam retrieves and converts a query parameter from an HTTP request to the specified
// type. If the query parameter is missing, the defaultValue is returned. The validators are optional
// and perform additional validation on the value.
//
//nolint:ireturn // Generic function must return type parameter T.
func GetQueryParam[T any](
	req *http.Request,
	key string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if key == "" {
		return defaultValue, ErrNoKey
	}

	return getFormValue[T](key, req.URL.Query().Get(key), defaultValue, validators...)
}

// getFormValue retrieves and converts a given value to the specified type T, with optional
// validation. The key names the value in any error returned.
//
//nolint:ireturn // Generic function must return type parameter T.
func getFormValue[T any](
	key string,
	formValue string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if formValue == "" {
		return defaultValue, nil
	}

	value, err := typeconv.ConvertTo[T](formValue)
	if err != nil {
		// Report the raw form value: the converted value is the zero value here, not the input
		// the caller needs to see.
		return defaultValue, &validate.ValidationError{Name: key, Value: formValue, Err: err}
	}

	err = validate.Validate(key, value, validators...)
	if err != nil {
		return defaultValue, err
	}

	return value, nil
}
