package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"unicode"

	"github.com/spacecafe/go-parts/pkg/typeconv"
)

var (
	_             Source = (*EnvSource)(nil)
	ErrConversion        = errors.New("failed to convert environment variable to field type")
)

// EnvSource loads configuration from environment variables.
type EnvSource struct {
	// Prefix is an optional application prefix for environment variables.
	// If set to "APP", it will look for variables like "APP_DATABASE_HOST".
	Prefix string
}

func (s EnvSource) Load(target any) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	valueOf := reflect.ValueOf(target).Elem()

	return s.loadStruct(valueOf, strings.ToUpper(s.Prefix))
}

// hasEnvWithPrefix checks if any environment variable with the given prefix exists.
func (s EnvSource) hasEnvWithPrefix(prefix string) bool {
	prefix += "_"
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			return true
		}
	}

	return false
}

// loadStruct recursively loads environment variables into struct fields.
func (s EnvSource) loadStruct(valueOf reflect.Value, prefix string) error {
	typeOf := valueOf.Type()

	for i := range valueOf.NumField() {
		field := valueOf.Field(i)
		fieldType := typeOf.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "-" {
			continue
		}

		// Build the environment variable name
		envName := createEnvName(prefix, fieldType.Name, envTag)

		// Handle nested structs recursively
		if field.Kind() == reflect.Struct {
			err := s.loadStruct(field, envName)
			if err != nil {
				return err
			}

			continue
		}

		// Handle pointers to structs
		//nolint:nestif // Required for optional nested struct initialization and loading.
		if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			// Initialize nil pointer if environment variable exists
			if s.hasEnvWithPrefix(envName) {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}

				err := s.loadStruct(field.Elem(), envName)
				if err != nil {
					return err
				}
			}

			continue
		}

		// Load the environment variable value
		envValue, exists := lookupEnv(envName)
		if !exists {
			continue
		}

		// Set the field value
		err := typeconv.Default.Convert(field, envValue)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrConversion, err)
		}
	}

	return nil
}

// createEnvName generates an environment variable name using the provided prefix, field name, and optional env tag.
// It converts camel case field names to uppercase with underscores and includes the prefix if provided.
func createEnvName(prefix, fieldName, envTag string) string {
	var result strings.Builder

	if prefix != "" {
		result.WriteString(prefix)
		result.WriteRune('_')
	}

	if envTag != "" {
		result.WriteString(strings.ToUpper(envTag))
	} else {
		for i, r := range fieldName {
			if i > 0 && r >= 'A' && r <= 'Z' {
				result.WriteRune('_')
			}

			result.WriteRune(unicode.ToUpper(r))
		}
	}

	return result.String()
}

func lookupEnv(envName string) (string, bool) {
	envValue, exists := os.LookupEnv(envName + "_FILE")
	if exists {
		data, err := os.ReadFile(path.Clean(strings.TrimSpace(envValue)))
		if err == nil {
			return strings.TrimSpace(string(data)), true
		}
	}

	envValue, exists = os.LookupEnv(envName)
	if exists {
		return strings.TrimSpace(envValue), true
	}

	return "", false
}
