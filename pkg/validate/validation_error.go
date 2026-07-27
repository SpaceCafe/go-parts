package validate

import (
	"fmt"
	"unicode/utf8"
)

// maxValueLength caps how much of a rejected value ends up in an error message. Validation
// errors travel into logs and HTTP responses, so an unbounded field would let a caller inflate
// both by sending a large body.
const maxValueLength = 64

// Redacted marks a value whose content must never appear in a validation error message.
// Implement it on named types carrying secrets (passwords, tokens, API keys) so the value is
// replaced by a placeholder instead of being echoed back to the client and written to logs.
type Redacted interface {
	Redacted()
}

// ValidationError reports which named value failed validation and why.
type ValidationError struct {
	Value any
	Err   error
	Name  string
}

func (r *ValidationError) Error() string {
	return fmt.Sprintf("%s (value %s): %s", r.Name, formatValue(r.Value), r.Err)
}

// Unwrap exposes the cause so errors.Is keeps matching the validate package's sentinels.
// A joined Err stays matchable because errors.Is walks the multi-error Unwrap of errors.Join.
func (r *ValidationError) Unwrap() error {
	return r.Err
}

// formatValue renders a rejected value for an error message, honoring Redacted and the cap.
func formatValue(value any) string {
	if _, ok := value.(Redacted); ok {
		return "<redacted>"
	}

	str, ok := value.(string)
	if !ok {
		str = fmt.Sprint(value)
	}

	formatted := fmt.Sprintf("%.*q", maxValueLength, str)

	if utf8.RuneCountInString(str) > maxValueLength {
		formatted += "..."
	}

	return formatted
}
