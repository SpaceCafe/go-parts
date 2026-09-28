package validate

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrLength        = errors.New("validate: value's length must be")
	ErrLengthBetween = errors.New("validate: value's length must be between")
	ErrLengthMax     = errors.New("validate: value's length must be less or equal than")
	ErrLengthMin     = errors.New("validate: value's length must be greater or equal than")
)

// Length validates that the provided string has the specified length in bytes. Use RuneLength to
// count characters.
func Length[T ~string](targetLength int) func(T) error {
	return length[any, T, T](targetLength)
}

// LengthBetween validates that the provided string has a length in bytes between the specified
// bounds. Use RuneLengthBetween to count characters.
func LengthBetween[T ~string](lowerBound, upperBound int) func(T) error {
	return lengthBetween[any, T, T](lowerBound, upperBound)
}

// LengthMin validates that the provided string has a length in bytes greater or equal than the
// specified bound. Use RuneLengthMin to count characters, for example for a password.
func LengthMin[T ~string](lowerBound int) func(T) error {
	return lengthMin[any, T, T](lowerBound)
}

// LengthMax validates that the provided string has a length in bytes less or equal than the
// specified bound, for example to fit a storage limit. Use RuneLengthMax to count characters.
func LengthMax[T ~string](upperBound int) func(T) error {
	return lengthMax[any, T, T](upperBound)
}

// RuneLength validates that the provided string has the specified number of characters (runes).
func RuneLength[T ~string](targetLength int) func(T) error {
	return func(value T) error {
		return checkLength(utf8.RuneCountInString(string(value)), targetLength)
	}
}

// RuneLengthBetween validates that the provided string has a number of characters (runes) between
// the specified bounds (inclusive).
func RuneLengthBetween[T ~string](lowerBound, upperBound int) func(T) error {
	return func(value T) error {
		return checkLengthBetween(utf8.RuneCountInString(string(value)), lowerBound, upperBound)
	}
}

// RuneLengthMin validates that the provided string has at least the specified number of characters
// (runes).
func RuneLengthMin[T ~string](lowerBound int) func(T) error {
	return func(value T) error {
		return checkLengthMin(utf8.RuneCountInString(string(value)), lowerBound)
	}
}

// RuneLengthMax validates that the provided string has at most the specified number of characters
// (runes).
func RuneLengthMax[T ~string](upperBound int) func(T) error {
	return func(value T) error {
		return checkLengthMax(utf8.RuneCountInString(string(value)), upperBound)
	}
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
		return checkLength(len(value), targetLength)
	}
}

// lengthBetween validates that the provided value has a length between the specified bounds (inclusive).
func lengthBetween[K comparable, V any, T ~string | ~[]V | ~map[K]V](
	lowerBound, upperBound int,
) func(T) error {
	return func(value T) error {
		return checkLengthBetween(len(value), lowerBound, upperBound)
	}
}

// lengthMin validates that the provided value has a length greater or equal than the specified bound.
func lengthMin[K comparable, V any, T ~string | ~[]V | ~map[K]V](lowerBound int) func(T) error {
	return func(value T) error {
		return checkLengthMin(len(value), lowerBound)
	}
}

// lengthMax validates that the provided value has a length less or equal than the specified bound.
func lengthMax[K comparable, V any, T ~string | ~[]V | ~map[K]V](upperBound int) func(T) error {
	return func(value T) error {
		return checkLengthMax(len(value), upperBound)
	}
}

// checkLength, checkLengthBetween, checkLengthMin and checkLengthMax compare a length that the
// caller counted in bytes, characters or elements, so every variant reports the same errors.
func checkLength(count, targetLength int) error {
	if count != targetLength {
		return fmt.Errorf("%w %v", ErrLength, targetLength)
	}

	return nil
}

func checkLengthBetween(count, lowerBound, upperBound int) error {
	if count < lowerBound || count > upperBound {
		return fmt.Errorf("%w %v and %v", ErrLengthBetween, lowerBound, upperBound)
	}

	return nil
}

func checkLengthMin(count, lowerBound int) error {
	if count < lowerBound {
		return fmt.Errorf("%w %d", ErrLengthMin, lowerBound)
	}

	return nil
}

func checkLengthMax(count, upperBound int) error {
	if count > upperBound {
		return fmt.Errorf("%w %d", ErrLengthMax, upperBound)
	}

	return nil
}
