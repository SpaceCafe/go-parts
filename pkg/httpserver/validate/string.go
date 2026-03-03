package validate

import "fmt"

var (
	ErrNotEmpty      = fmt.Errorf("validate: value cannot be empty")
	ErrLengthBetween = fmt.Errorf("validate: value's length must be between")
	ErrLengthMax     = fmt.Errorf("validate: value's length must be less or equal than")
	ErrLengthMin     = fmt.Errorf("validate: value's length must be greater or equal than")
)

func NotEmpty(value string) error {
	if value == "" {
		return ErrNotEmpty
	}

	return nil
}

// LengthBetween validates if a string's length is between the specified bounds (inclusive).
func LengthBetween(lowerBound, upperBound int) func(string) error {
	return func(value string) error {
		if len(value) < lowerBound || len(value) > upperBound {
			return fmt.Errorf("%w %v and %v", ErrLengthBetween, lowerBound, upperBound)
		}

		return nil
	}
}

// LengthMax ensures a string's length does not exceed the specified upperBound.
func LengthMax(upperBound int) func(string) error {
	return func(value string) error {
		if len(value) > upperBound {
			return fmt.Errorf("%w %d", ErrLengthMax, upperBound)
		}

		return nil
	}
}

// LengthMin ensures a string's length is at least the specified lowerBound.
func LengthMin(lowerBound int) func(string) error {
	return func(value string) error {
		if len(value) < lowerBound {
			return fmt.Errorf("%w %d", ErrLengthMin, lowerBound)
		}

		return nil
	}
}
