package validate

import (
	"errors"
)

var (
	ErrError   = errors.New("validate: value must be an error")
	ErrNoError = errors.New("validate: value must not be an error")
)

// Error validates that the provided error is not nil.
func Error(err error) error {
	if err == nil {
		return ErrError
	}

	return nil
}

// NoError validates that the provided error is nil.
func NoError(err error) error {
	if err != nil {
		return ErrNoError
	}

	return nil
}
