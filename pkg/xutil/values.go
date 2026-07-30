package xutil

import (
	"cmp"
)

// Must panics if err is not nil, otherwise it returns value.
//
//nolint:ireturn // Generic function has to return type T.
func Must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}

	return value
}

// Clamp restricts value to the inclusive range [lowerBoundary, upperBoundary].
//
//nolint:ireturn // Generic function has to return type T.
func Clamp[T cmp.Ordered](lowerBoundary, value, upperBoundary T) T {
	return max(lowerBoundary, min(value, upperBoundary))
}

// Default returns value if it's not the zero value of its type,
// otherwise it returns defaultValue.
//
//nolint:ireturn // Generic function has to return type T.
func Default[T comparable](value, defaultValue T) T {
	var zero T

	if value == zero {
		return defaultValue
	}

	return value
}

// DefaultSlice returns value if it's not empty, otherwise it returns defaultValue.
//

func DefaultSlice[T any](value, defaultValue []T) []T {
	if len(value) == 0 {
		return defaultValue
	}

	return value
}

// DefaultMap returns value if it's not empty, otherwise it returns defaultValue.
//

func DefaultMap[K comparable, V any](value, defaultValue map[K]V) map[K]V {
	if len(value) == 0 {
		return defaultValue
	}

	return value
}
