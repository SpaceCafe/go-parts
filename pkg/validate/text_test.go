package validate_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
)

func TestAllowedSymbols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
		list    []rune
	}{
		{name: "all symbols allowed", list: []rune{'a', 'b', 'c'}, value: "abcba"},
		{name: "empty value", list: []rune{'a'}, value: ""},
		{
			name:    "one symbol outside list",
			list:    []rune{'a', 'b'},
			value:   "abc",
			wantErr: validate.ErrAllowedSymbols,
		},
		{name: "empty list", list: nil, value: "a", wantErr: validate.ErrAllowedSymbols},
		{name: "multi byte rune allowed", list: []rune{'ä'}, value: "ää"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.AllowedSymbols[string](tt.list)(tt.value))
		})
	}
}

func TestAllowedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
		list    []string
	}{
		{name: "value in list", list: []string{"debug", "info"}, value: "info"},
		{
			name:    "value not in list",
			list:    []string{"debug", "info"},
			value:   "warn",
			wantErr: validate.ErrAllowedValues,
		},
		{name: "empty list", list: nil, value: "info", wantErr: validate.ErrAllowedValues},
		{
			name:    "case sensitive",
			list:    []string{"info"},
			value:   "INFO",
			wantErr: validate.ErrAllowedValues,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.AllowedValues(tt.list)(tt.value))
		})
	}
}

func TestFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "name with extension", value: "config.yaml"},
		{name: "dashes and underscores", value: "my-file_1.txt"},
		{name: "single character", value: "a"},
		{name: "leading dot", value: ".env"},
		{name: "path separator", value: "dir/file", wantErr: validate.ErrAllowedSymbols},
		{name: "parent traversal", value: "../etc", wantErr: validate.ErrAllowedSymbols},
		{name: "empty", value: "", wantErr: validate.ErrAllowedSymbols},
		{name: "space", value: "my file", wantErr: validate.ErrAllowedSymbols},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Filename(tt.value))
		})
	}
}

func TestHex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "lower case digits", value: "deadbeef"},
		{name: "upper case digits", value: "DEADBEEF"},
		{name: "mixed case and numbers", value: "0aF9"},
		{name: "empty", value: ""},
		{name: "letter outside range", value: "xyz", wantErr: validate.ErrAllowedSymbols},
		{name: "hex prefix", value: "0xff", wantErr: validate.ErrAllowedSymbols},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Hex(tt.value))
		})
	}
}

func TestMatchRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		regex   string
		value   string
	}{
		{name: "anchored match", regex: `^v\d+$`, value: "v2"},
		{name: "no match", regex: `^v\d+$`, value: "2", wantErr: validate.ErrAllowedSymbols},
		{name: "unanchored pattern", regex: `\d+`, value: "abc123def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.MatchRegex[string](tt.regex)(tt.value))
		})
	}
}

func TestPrintableASCII(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "letters and digits", value: "hello 123"},
		{name: "punctuation", value: "~!@#$%^&*()_+"},
		{name: "empty", value: ""},
		{name: "tab", value: "a\tb", wantErr: validate.ErrAllowedSymbols},
		{name: "newline", value: "a\nb", wantErr: validate.ErrAllowedSymbols},
		{name: "null byte", value: "a\x00b", wantErr: validate.ErrAllowedSymbols},
		{name: "non ascii rune", value: "café", wantErr: validate.ErrAllowedSymbols},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.PrintableASCII(tt.value))
		})
	}
}
