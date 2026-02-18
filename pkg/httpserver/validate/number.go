package validate

import (
	"errors"
	"fmt"
)

var (
	ErrPositive    = errors.New("validate: value must be positive")
	ErrBetween     = errors.New("validate: value must be between")
	ErrNegative    = errors.New("validate: value must be negative")
	ErrNonNegative = errors.New("validate: value must be non-negative")
	ErrMax         = errors.New("validate: value must be less or equal than")
	ErrMin         = errors.New("validate: value must be greater or equal than")
	ErrDivisorZero = errors.New("validate: divisor cannot be zero")
	ErrMultipleOf  = errors.New("validate: value must be multiple of")
)

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~uintptr
}

type Number interface {
	Integer | ~float32 | ~float64
}

// Between validates if a value is within the specified lower and upper bounds (inclusive).
func Between[T Number](value, lowerBound, upperBound T) error {
	if value < lowerBound || value > upperBound {
		return fmt.Errorf("%w %v and %v", ErrBetween, lowerBound, upperBound)
	}

	return nil
}

// Max validates if a value is less than or equal to the specified upper bound.
func Max[T Number](value, upperBound T) error {
	if value > upperBound {
		return fmt.Errorf("%w %v", ErrMax, upperBound)
	}

	return nil
}

// Min validates if a value is greater than or equal to the specified lower bound.
func Min[T Number](value, lowerBound T) error {
	if value < lowerBound {
		return fmt.Errorf("%w %v", ErrMin, lowerBound)
	}

	return nil
}

// MultipleOf validates if a value is a multiple of the specified divisor.
func MultipleOf[T Integer](value, divisor T) error {
	if divisor == 0 {
		return ErrDivisorZero
	}

	if value%divisor != 0 {
		return fmt.Errorf("%w %v", ErrMultipleOf, divisor)
	}

	return nil
}

// Negative checks if the given numeric value is negative.
func Negative[T Number](value T) error {
	if value >= 0 {
		return ErrNegative
	}

	return nil
}

// NonNegative validates that the provided numeric value is non-negative (including zero).
func NonNegative[T Number](value T) error {
	if value < 0 {
		return ErrNonNegative
	}

	return nil
}

// Positive checks if the provided value is positive (excluding zero).
func Positive[T Number](value T) error {
	if value <= 0 {
		return ErrPositive
	}

	return nil
}
