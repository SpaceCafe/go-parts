package validate

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrAllowedSymbols  = errors.New("validate: value must contain only allowed symbols")
	ErrAllowedValues   = errors.New("validate: value must be one of allowed values")
	ErrInvalidFilename = errors.New(
		`validate: filename must not be "." or ".." or start with "-"`,
	)

	filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)
)

// AllowedSymbols returns a validation function that checks if a string contains only the specified allowed symbols.
func AllowedSymbols[T ~string](list []rune) func(T) error {
	return func(value T) error {
		for _, char := range value {
			if !slices.Contains(list, char) {
				return fmt.Errorf("%w %v", ErrAllowedSymbols, list)
			}
		}

		return nil
	}
}

// AllowedValues returns a validator that accepts only values present in list.
func AllowedValues[T comparable](list []T) func(T) error {
	return func(value T) error {
		if !slices.Contains(list, value) {
			return fmt.Errorf("%w %v", ErrAllowedValues, list)
		}

		return nil
	}
}

// Filename validates that value is a single, safe filename: only letters, digits, "-", "_" and ".".
// It rejects "." and "..", which would refer to the current or parent directory when joined into a
// path, and a leading "-", which a command-line tool would parse as an option. Names starting with
// "." (such as ".env") are allowed.
func Filename[T ~string](value T) error {
	name := string(value)

	if !filenameRegex.MatchString(name) {
		return fmt.Errorf("%w %s", ErrAllowedSymbols, filenameRegex.String())
	}

	if name == "." || name == ".." || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %q", ErrInvalidFilename, name)
	}

	return nil
}

// Hex checks if the given string contains only valid hexadecimal characters (0-9, A-F, a-f).
func Hex[T ~string](value T) error {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'A' || char > 'F') && (char < 'a' || char > 'f') {
			return fmt.Errorf("%w 0-9,A-F,a-f", ErrAllowedSymbols)
		}
	}

	return nil
}

// MatchRegex validates a string against a regular expression.
func MatchRegex[T ~string](regex string) func(T) error {
	pattern := regexp.MustCompile(regex)

	return func(value T) error {
		if !pattern.MatchString(string(value)) {
			return fmt.Errorf("%w %s", ErrAllowedSymbols, regex)
		}

		return nil
	}
}

// PrintableASCII checks if the input string contains non-printable ASCII characters (ASCII codes outside 32-126).
func PrintableASCII[T ~string](value T) error {
	for _, char := range value {
		if char < 32 || char > 126 {
			return fmt.Errorf("%w char 32-126", ErrAllowedSymbols)
		}
	}

	return nil
}
