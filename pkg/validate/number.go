package validate

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrNotPositive    = errors.New("validate: value must be positive")
	ErrNotBetween     = errors.New("validate: value must be between")
	ErrNotNegative    = errors.New("validate: value must be negative")
	ErrNotNonNegative = errors.New("validate: value must be non-negative")
	ErrMax            = errors.New("validate: value must be less or equal than")
	ErrMin            = errors.New("validate: value must be greater or equal than")
	ErrDivisorZero    = errors.New("validate: divisor cannot be zero")
	ErrNotMultipleOf  = errors.New("validate: value must be multiple of")
)

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~uintptr
}

type Number interface {
	Integer | ~float32 | ~float64
}

// Between validates that the provided value is within the specified lower and upper bounds (inclusive).
func Between[T Number](lowerBound, upperBound T) func(T) error {
	return func(value T) error {
		if value < lowerBound || value > upperBound {
			return fmt.Errorf("%w %v and %v", ErrNotBetween, lowerBound, upperBound)
		}

		return nil
	}
}

// Max validates that the provided value is less than or equal to the specified upper bound.
func Max[T Number](upperBound T) func(T) error {
	return func(value T) error {
		if value > upperBound {
			return fmt.Errorf("%w %v", ErrMax, upperBound)
		}

		return nil
	}
}

// Min validates that the provided value is greater than or equal to the specified lower bound.
func Min[T Number](lowerBound T) func(T) error {
	return func(value T) error {
		if value < lowerBound {
			return fmt.Errorf("%w %v", ErrMin, lowerBound)
		}

		return nil
	}
}

// MultipleOf validates that the provided value is a multiple of the specified divisor.
func MultipleOf[T Integer](divisor T) func(T) error {
	return func(value T) error {
		if divisor == 0 {
			return ErrDivisorZero
		}

		if value%divisor != 0 {
			return fmt.Errorf("%w %v", ErrNotMultipleOf, divisor)
		}

		return nil
	}
}

// Negative validates that the provided numeric value is negative.
func Negative[T Number](value T) error {
	if value >= 0 {
		return ErrNotNegative
	}

	return nil
}

// NonNegative validates that the provided numeric value is non-negative (including zero).
func NonNegative[T Number](value T) error {
	if value < 0 {
		return ErrNotNonNegative
	}

	return nil
}

// Port validates that the provided numeric value is a valid port number (between 0 and 65535).
func Port[T Number](value T) error {
	return Between(0, math.MaxUint16)(int(value))
}

// Positive validates that the provided numeric value is positive (excluding zero).
func Positive[T Number](value T) error {
	if value <= 0 {
		return ErrNotPositive
	}

	return nil
}
