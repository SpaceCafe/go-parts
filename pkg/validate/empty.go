package validate

import "errors"

var (
	ErrEmpty    = errors.New("validate: value must be empty")
	ErrNotEmpty = errors.New("validate: value must not be empty")
)

// Empty validates that the provided string is empty.
func Empty[T ~string](value T) error {
	if value != "" {
		return ErrEmpty
	}

	return nil
}

// EmptyMap validates that the provided map is empty.
func EmptyMap[K comparable, V any](value map[K]V) error {
	if len(value) > 0 {
		return ErrEmpty
	}

	return nil
}

// EmptySlice validates that the provided slice is empty.
func EmptySlice[V any](value []V) error {
	if len(value) > 0 {
		return ErrEmpty
	}

	return nil
}

// NotEmpty validates that the provided string is not empty.
func NotEmpty[T ~string](value T) error {
	if value == "" {
		return ErrNotEmpty
	}

	return nil
}

// NotEmptyMap validates that the provided map is not empty.
func NotEmptyMap[K comparable, V any](value map[K]V) error {
	if len(value) == 0 {
		return ErrNotEmpty
	}

	return nil
}

// NotEmptySlice validates that the provided slice is not empty.
func NotEmptySlice[V any](value []V) error {
	if len(value) == 0 {
		return ErrNotEmpty
	}

	return nil
}
