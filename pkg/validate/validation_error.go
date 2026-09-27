package validate

import (
	"fmt"
	"reflect"
	"unicode/utf8"
)

// maxValueLength caps how much of a rejected value ends up in an error message. Validation
// errors travel into logs and HTTP responses, so an unbounded field would let a caller inflate
// both by sending a large body.
const maxValueLength = 64

// redactedPlaceholder replaces redacted values in error messages.
const redactedPlaceholder = "<redacted>"

// Redacted marks a value whose content must never appear in a validation error message.
// Implement it on named types carrying secrets (passwords, tokens, API keys) so the value is
// replaced by a placeholder instead of being echoed back to the client and written to logs.
type Redacted interface {
	Redacted()
}

// redactedType is the reflect.Type of Redacted, used to find it inside container types.
//
//nolint:gochecknoglobals // Constant reflect.Type, computed once.
var redactedType = reflect.TypeFor[Redacted]()

// Secret is a string that implements Redacted. Use it for passwords, tokens and API keys in
// configuration structs. String also returns a placeholder, so fmt and log output that print the
// value directly, or any container holding it, do not reveal it either.
type Secret string

// Redacted marks Secret as redacted, satisfying the Redacted interface.
func (Secret) Redacted() {}

// String returns a placeholder instead of the secret. Convert with string(secret) to get the value.
func (Secret) String() string {
	return redactedPlaceholder
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

// formatValue renders a rejected value for an error message, honoring Redacted and the cap. A
// container (map, slice, array, pointer or struct) that can hold a Redacted value is redacted as a
// whole, so a map of passwords does not print its entries.
func formatValue(value any) string {
	if _, ok := value.(Redacted); ok {
		return redactedPlaceholder
	}

	if value != nil && holdsRedacted(reflect.TypeOf(value), make(map[reflect.Type]bool)) {
		return redactedPlaceholder
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

// holdsRedacted reports whether typ implements Redacted or can contain a value that does. seen
// stops recursion on self-referential types.
func holdsRedacted(typ reflect.Type, seen map[reflect.Type]bool) bool {
	if typ.Implements(redactedType) {
		return true
	}

	if seen[typ] {
		return false
	}

	seen[typ] = true

	//nolint:exhaustive // Only these kinds can contain other values; every other kind is a leaf.
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return holdsRedacted(typ.Elem(), seen)
	case reflect.Map:
		return holdsRedacted(typ.Key(), seen) || holdsRedacted(typ.Elem(), seen)
	case reflect.Struct:
		for field := range typ.Fields() {
			if holdsRedacted(field.Type, seen) {
				return true
			}
		}
	}

	return false
}
