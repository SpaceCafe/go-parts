package validate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/require"
)

// maxValueLength mirrors the unexported cap in the package under test, so the truncation cases can
// be expressed in terms of it instead of a bare number.
const maxValueLength = 64

// secret stands in for a config field carrying a credential. Implementing Redacted is all it takes
// to keep the content out of error messages.
type secret string

func (secret) Redacted() {}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value any
		name  string
		want  string
	}{
		{
			name:  "string value",
			value: "bob",
			want:  `username (value "bob"): validate: value must be empty`,
		},
		{
			name:  "non string value",
			value: 8080,
			want:  `username (value "8080"): validate: value must be empty`,
		},
		{
			name:  "empty value",
			value: "",
			want:  `username (value ""): validate: value must be empty`,
		},
		{
			name:  "multi byte runes are kept intact",
			value: "café",
			want:  `username (value "café"): validate: value must be empty`,
		},
		{
			name:  "redacted value",
			value: secret("hunter2"),
			want:  `username (value <redacted>): validate: value must be empty`,
		},
		{
			name:  "value on the cap",
			value: strings.Repeat("a", maxValueLength),
			want: `username (value "` + strings.Repeat(
				"a",
				maxValueLength,
			) + `"): validate: value must be empty`,
		},
		{
			name:  "value beyond the cap",
			value: strings.Repeat("a", maxValueLength+10),
			want: `username (value "` + strings.Repeat("a", maxValueLength) +
				`"...): validate: value must be empty`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &validate.ValidationError{
				Name:  "username",
				Value: tt.value,
				Err:   validate.ErrNotEmpty,
			}

			require.Equal(t, tt.want, err.Error())
		})
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("single cause", func(t *testing.T) {
		t.Parallel()

		err := &validate.ValidationError{Name: "username", Value: "", Err: validate.ErrNotEmpty}

		require.Equal(t, validate.ErrNotEmpty, err.Unwrap())
		require.ErrorIs(t, err, validate.ErrNotEmpty)
	})

	t.Run("joined causes", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(validate.ErrNotEmpty, validate.ErrLengthMin)
		err := &validate.ValidationError{Name: "username", Value: "", Err: joined}

		require.ErrorIs(t, err, validate.ErrNotEmpty)
		require.ErrorIs(t, err, validate.ErrLengthMin)
		require.NotErrorIs(t, err, validate.ErrNil)
	})
}
