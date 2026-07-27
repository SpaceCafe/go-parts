package validate

import (
	"errors"
	"fmt"
)

// Validate runs every validator against value and joins their errors. The name identifies the
// value to the caller, for example, a query parameter or a struct field. A non-nil result is
// always a *ValidationError.
func Validate[T any](name string, value T, validators ...func(T) error) error {
	errs := make([]error, len(validators))
	for i, validator := range validators {
		if validator == nil {
			continue
		}

		errs[i] = validator(value)
	}

	// Wrapping the joined error once, rather than each validator's error, keeps the name and
	// value from repeating on every line when several validators fail on the same input.
	err := errors.Join(errs...)
	if err != nil {
		return &ValidationError{Name: name, Value: value, Err: err}
	}

	return nil
}

// ValidateSlice runs every validator against each element of values. Errors are reported against
// `name[i]` so the caller can tell which element was rejected.
func ValidateSlice[S ~[]E, E any](name string, values S, validators ...func(E) error) error {
	errs := make([]error, len(values))
	for i, value := range values {
		errs[i] = Validate(fmt.Sprintf("%s[%d]", name, i), value, validators...)
	}

	return errors.Join(errs...)
}

// ValidateMap validates all entries in a map using the provided validators and returns any combined validation errors.
func ValidateMap[M ~map[K]V, K comparable, V any](name string, values M, validators ...func(V) error) error {
	errs := make([]error, len(values))
	i := 0
	for key, value := range values {
		errs[i] = Validate(fmt.Sprintf("%s[%v]", name, key), value, validators...)
		i++
	}

	return errors.Join(errs...)
}
