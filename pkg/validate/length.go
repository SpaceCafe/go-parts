package validate

import (
	"errors"
	"fmt"
)

var (
	ErrLength        = errors.New("validate: value's length must be")
	ErrLengthBetween = errors.New("validate: value's length must be between")
	ErrLengthMax     = errors.New("validate: value's length must be less or equal than")
	ErrLengthMin     = errors.New("validate: value's length must be greater or equal than")
)

// Length validates that the provided string has the specified length.
func Length[T ~string](targetLength int) func(T) error {
	return length[any, T, T](targetLength)
}

// LengthBetween validates that the provided string has a length between the specified bounds.
func LengthBetween[T ~string](lowerBound, upperBound int) func(T) error {
	return lengthBetween[any, T, T](lowerBound, upperBound)
}

// LengthMin validates that the provided string has a length greater or equal than the specified bound.
func LengthMin[T ~string](lowerBound int) func(T) error {
	return lengthMin[any, T, T](lowerBound)
}

// LengthMax validates that the provided string has a length less or equal than the specified bound.
func LengthMax[T ~string](upperBound int) func(T) error {
	return lengthMax[any, T, T](upperBound)
}

// MapLength validates that the provided map has the specified length.
func MapLength[K comparable, V any](targetLength int) func(map[K]V) error {
	return length[K, V, map[K]V](targetLength)
}

// MapLengthBetween validates that the provided map has a length between the specified bounds.
func MapLengthBetween[K comparable, V any](lowerBound, upperBound int) func(map[K]V) error {
	return lengthBetween[K, V, map[K]V](lowerBound, upperBound)
}

// MapLengthMin validates that the provided map has a length greater or equal than the specified bound.
func MapLengthMin[K comparable, V any](lowerBound int) func(map[K]V) error {
	return lengthMin[K, V, map[K]V](lowerBound)
}

// MapLengthMax validates that the provided map has a length less or equal than the specified bound.
func MapLengthMax[K comparable, V any](upperBound int) func(map[K]V) error {
	return lengthMax[K, V, map[K]V](upperBound)
}

// SliceLength validates that the provided slice has the specified length.
func SliceLength[V any](targetLength int) func([]V) error {
	return length[any, V, []V](targetLength)
}

// SliceLengthBetween validates that the provided slice has a length between the specified bounds.
func SliceLengthBetween[V any](lowerBound, upperBound int) func([]V) error {
	return lengthBetween[any, V, []V](lowerBound, upperBound)
}

// SliceLengthMin validates that the provided slice has a length greater or equal than the specified bound.
func SliceLengthMin[V any](lowerBound int) func([]V) error {
	return lengthMin[any, V, []V](lowerBound)
}

// SliceLengthMax validates that the provided slice has a length less or equal than the specified bound.
func SliceLengthMax[V any](upperBound int) func([]V) error {
	return lengthMax[any, V, []V](upperBound)
}

// length validates that the provided value has the specified length.
func length[K comparable, V any, T ~string | ~[]V | ~map[K]V](targetLength int) func(T) error {
	return func(value T) error {
		if len(value) != targetLength {
			return fmt.Errorf("%w %v", ErrLength, targetLength)
		}

		return nil
	}
}

// lengthBetween validates that the provided value has a length between the specified bounds (inclusive).
func lengthBetween[K comparable, V any, T ~string | ~[]V | ~map[K]V](lowerBound, upperBound int) func(T) error {
	return func(value T) error {
		if len(value) < lowerBound || len(value) > upperBound {
			return fmt.Errorf("%w %v and %v", ErrLengthBetween, lowerBound, upperBound)
		}

		return nil
	}
}

// lengthMin validates that the provided value has a length greater or equal than the specified bound.
func lengthMin[K comparable, V any, T ~string | ~[]V | ~map[K]V](lowerBound int) func(T) error {
	return func(value T) error {
		if len(value) < lowerBound {
			return fmt.Errorf("%w %d", ErrLengthMin, lowerBound)
		}

		return nil
	}
}

// lengthMax validates that the provided value has a length less or equal than the specified bound.
func lengthMax[K comparable, V any, T ~string | ~[]V | ~map[K]V](upperBound int) func(T) error {
	return func(value T) error {
		if len(value) > upperBound {
			return fmt.Errorf("%w %d", ErrLengthMax, upperBound)
		}

		return nil
	}
}
