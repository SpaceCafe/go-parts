package validate

import "errors"

var (
	ErrNil    = errors.New("validate: value must be nil")
	ErrNotNil = errors.New("validate: value must not be nil")
)

// NilMap validates that the provided map is nil.
func NilMap[K comparable, V any](value map[K]V) error {
	if value == nil {
		return ErrNil
	}

	return nil
}

// NilPointer validates that the provided pointer is nil.
func NilPointer[T any](value *T) error {
	if value != nil {
		return ErrNil
	}

	return nil
}

// NilSlice validates that the provided slice is nil.
func NilSlice[V any](value []V) error {
	if value == nil {
		return ErrNil
	}

	return nil
}

// NotNilMap validates that the provided map is not nil.
func NotNilMap[K comparable, V any](value map[K]V) error {
	if value == nil {
		return ErrNotNil
	}

	return nil
}

// NotNilPointer validates that the provided pointer is not nil.
func NotNilPointer[T any](value *T) error {
	if value == nil {
		return ErrNotNil
	}

	return nil
}

// NotNilSlice validates that the provided slice is not nil.
func NotNilSlice[V any](value []V) error {
	if value == nil {
		return ErrNotNil
	}

	return nil
}
