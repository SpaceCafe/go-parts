package config

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrInvalidTarget  = errors.New("invalid target")
	ErrConfigNotFound = errors.New("config not found")
	ErrInvalidConfig  = errors.New("invalid config")
)

// Validatable ensures that the configuration struct provides a validation method.
type Validatable interface {
	Validate() error
}

// Source defines a configuration source.
type Source interface {
	Load(target any) error
}

// Load loads configuration from multiple sources and validates the result.
// This is simpler than using a Loader struct for this straightforward operation.
//
//nolint:wrapcheck // Errors are already wrapped in sources.
func Load(target Validatable, sources ...Source) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	for _, s := range sources {
		err = s.Load(target)
		if err != nil {
			return err
		}
	}

	return target.Validate()
}

// validatePointerToStruct ensures the target is a non-nil pointer to a struct.
func validatePointerToStruct(target any) error {
	if target == nil {
		return fmt.Errorf("%w: target cannot be nil", ErrInvalidTarget)
	}

	valueOf := reflect.ValueOf(target)
	if valueOf.Kind() != reflect.Ptr {
		return fmt.Errorf(
			"%w: target must be a pointer, got %T",
			ErrInvalidTarget,
			target,
		)
	}

	if valueOf.IsNil() {
		return fmt.Errorf("%w: target pointer cannot be nil", ErrInvalidTarget)
	}

	if valueOf.Elem().Kind() != reflect.Struct {
		return fmt.Errorf(
			"%w: target must be a pointer to struct, got pointer to %s",
			ErrInvalidTarget,
			valueOf.Elem().Kind(),
		)
	}

	return nil
}
