package validate_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/require"
)

func TestElements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    []string
		wantErrs []error
	}{
		{name: "all elements valid", value: []string{"a", "b"}},
		{name: "empty slice", value: nil},
		{
			name:     "one element invalid",
			value:    []string{"a", ""},
			wantErrs: []error{validate.ErrEmpty},
		},
		{
			name:     "several elements invalid",
			value:    []string{"", ""},
			wantErrs: []error{validate.ErrEmpty},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.Elements(validate.NotEmpty[string])(tt.value)

			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)

				return
			}

			for _, wantErr := range tt.wantErrs {
				require.ErrorIs(t, err, wantErr)
			}
		})
	}
}

// TestElementsNamesIndex pins down that a failing element is identified by its position, which is
// the only handle a caller has on an unnamed slice entry.
func TestElementsNamesIndex(t *testing.T) {
	t.Parallel()

	err := validate.Elements(validate.NotEmpty[string])([]string{"a", ""})

	require.ErrorContains(t, err, "1 (value \"\")")
	require.NotContains(t, err.Error(), "0 (value")
}

func TestEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value    map[string]string
		name     string
		wantErrs []error
	}{
		{name: "all entries valid", value: map[string]string{"host": "localhost"}},
		{name: "empty map", value: nil},
		{
			name:     "one entry invalid",
			value:    map[string]string{"host": ""},
			wantErrs: []error{validate.ErrEmpty},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.Entries[string](validate.NotEmpty[string])(tt.value)

			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)

				return
			}

			for _, wantErr := range tt.wantErrs {
				require.ErrorIs(t, err, wantErr)
			}
		})
	}
}

// TestEntriesNamesKey pins down that a failing entry is identified by its key. Only one entry is
// invalid because map iteration order is unspecified, so the order of several messages is not.
func TestEntriesNamesKey(t *testing.T) {
	t.Parallel()

	err := validate.Entries[string](validate.NotEmpty[string])(map[string]string{"host": ""})

	require.ErrorContains(t, err, "host (value \"\")")
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		validators []func(string) error
		wantErrs   []error
	}{
		{name: "no validators", value: ""},
		{name: "all validators pass", value: "abc", validators: []func(string) error{
			validate.NotEmpty[string],
			validate.LengthMax[string](3),
		}},
		{
			name:       "single failure",
			value:      "",
			validators: []func(string) error{validate.NotEmpty[string]},
			wantErrs:   []error{validate.ErrEmpty},
		},
		{
			name:  "every failure is reported",
			value: "",
			validators: []func(string) error{
				validate.NotEmpty[string],
				validate.LengthMin[string](3),
			},
			wantErrs: []error{validate.ErrEmpty, validate.ErrLengthMin},
		},
		{
			name:       "nil validator is skipped",
			value:      "abc",
			validators: []func(string) error{nil, validate.NotEmpty[string], nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.Validate("field", tt.value, tt.validators...)

			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)

				return
			}

			for _, wantErr := range tt.wantErrs {
				require.ErrorIs(t, err, wantErr)
			}
		})
	}
}

// TestValidateReturnsValidationError pins down the documented promise that a non-nil result is
// always a *ValidationError carrying the name and value the caller passed in.
func TestValidateReturnsValidationError(t *testing.T) {
	t.Parallel()

	err := validate.Validate("username", "", validate.NotEmpty[string])

	var validationErr *validate.ValidationError

	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, "username", validationErr.Name)
	require.Empty(t, validationErr.Value)
	require.ErrorIs(t, err, validate.ErrEmpty)
}
