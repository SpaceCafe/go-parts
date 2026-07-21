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
	if err := errors.Join(errs...); err != nil {
		return &ValidationError{Name: name, Value: value, Err: err}
	}

	return nil
}

// ValidateSlice runs every validator against each element of values. Errors are reported against
// `name[i]` so the caller can tell which element was rejected.
func ValidateSlice[T any](name string, values []T, validators ...func(T) error) error {
	errs := make([]error, len(values))
	for i, value := range values {
		errs[i] = Validate(fmt.Sprintf("%s[%d]", name, i), value, validators...)
	}

	return errors.Join(errs...)
}
