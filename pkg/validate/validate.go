package validate

import (
	"errors"
	"fmt"
	"strconv"
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

// Elements validates each element of a slice using the provided validators and returns
// any combined validation errors.
func Elements[T any](validators ...func(T) error) func([]T) error {
	return func(values []T) error {
		errs := make([]error, len(values))
		for i, value := range values {
			errs[i] = Validate(strconv.Itoa(i), value, validators...)
		}

		return errors.Join(errs...)
	}
}

// Entries validates each value of a map using the provided validators and returns
// any combined validation errors.
func Entries[K comparable, V any](validators ...func(V) error) func(map[K]V) error {
	return func(values map[K]V) error {
		errs := make([]error, len(values))

		i := 0
		for key, value := range values {
			errs[i] = Validate(fmt.Sprintf("%v", key), value, validators...)
			i++
		}

		return errors.Join(errs...)
	}
}
