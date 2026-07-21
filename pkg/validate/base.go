package validate

import (
	"errors"
)

var ErrNotNil = errors.New("validate: value must be nil")

// Nil validates if the given value is nil.
func Nil(value any) error {
	if value != nil {
		return ErrNotNil
	}

	return nil
}

// NoError returns nil if the provided error is nil or the given error.
func NoError(err error) error {
	if err != nil {
		return err
	}

	return nil
}
