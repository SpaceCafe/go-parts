package validate

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

var (
	ErrAllowedSymbols = errors.New("validate: value must contain only allowed symbols")
	ErrLengthBetween  = errors.New("validate: value's length must be between")
	ErrLengthMax      = errors.New("validate: value's length must be less or equal than")
	ErrLengthMin      = errors.New("validate: value's length must be greater or equal than")
	ErrNotEmpty       = errors.New("validate: value cannot be empty")

	filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]+[a-zA-Z0-9\-_.]+$`)
)

func AllowedSymbols(list []rune) func(string) error {
	return func(value string) error {
		for _, char := range value {
			if !slices.Contains(list, char) {
				return fmt.Errorf("%w %v", ErrAllowedSymbols, list)
			}
		}

		return nil
	}
}

func Filename(value string) error {
	if !filenameRegex.MatchString(value) {
		return fmt.Errorf("%w %v", ErrAllowedSymbols, filenameRegex.String())
	}

	return nil
}

func Hex(value string) error {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'A' || char > 'F') && (char < 'a' || char > 'f') {
			return fmt.Errorf("%w 0-9,A-F,a-f", ErrAllowedSymbols)
		}
	}

	return nil
}

// LengthBetween validates if a string's length is between the specified bounds (inclusive).
func LengthBetween(lowerBound, upperBound int) func(string) error {
	return func(value string) error {
		if len(value) < lowerBound || len(value) > upperBound {
			return fmt.Errorf("%w %v and %v", ErrLengthBetween, lowerBound, upperBound)
		}

		return nil
	}
}

// LengthMax ensures a string's length does not exceed the specified upperBound.
func LengthMax(upperBound int) func(string) error {
	return func(value string) error {
		if len(value) > upperBound {
			return fmt.Errorf("%w %d", ErrLengthMax, upperBound)
		}

		return nil
	}
}

// LengthMin ensures a string's length is at least the specified lowerBound.
func LengthMin(lowerBound int) func(string) error {
	return func(value string) error {
		if len(value) < lowerBound {
			return fmt.Errorf("%w %d", ErrLengthMin, lowerBound)
		}

		return nil
	}
}

func MatchRegex(regex string) func(string) error {
	pattern := regexp.MustCompile(regex)

	return func(value string) error {
		if !pattern.MatchString(value) {
			return fmt.Errorf("%w %s", ErrAllowedSymbols, regex)
		}

		return nil
	}
}

func NotEmpty(value string) error {
	if value == "" {
		return ErrNotEmpty
	}

	return nil
}

func PrintableASCII(value string) error {
	for _, char := range value {
		if char < 32 || char > 126 {
			return fmt.Errorf("%w char 32-126", ErrAllowedSymbols)
		}
	}

	return nil
}
