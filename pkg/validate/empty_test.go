package validate_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
)

func TestEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty", value: ""},
		{name: "single character", value: "x", wantErr: validate.ErrNotEmpty},
		{name: "whitespace only", value: " ", wantErr: validate.ErrNotEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Empty(tt.value))
		})
	}
}

func TestEmptyMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   map[string]int
		wantErr error
		name    string
	}{
		{name: "nil map", value: nil},
		{name: "empty map", value: map[string]int{}},
		{name: "one entry", value: map[string]int{"a": 1}, wantErr: validate.ErrNotEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.EmptyMap(tt.value))
		})
	}
}

func TestEmptySlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   []int
	}{
		{name: "nil slice", value: nil},
		{name: "empty slice", value: []int{}},
		{name: "one element", value: []int{1}, wantErr: validate.ErrNotEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.EmptySlice(tt.value))
		})
	}
}

func TestNotEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "single character", value: "x"},
		{name: "whitespace only", value: " "},
		{name: "empty", value: "", wantErr: validate.ErrEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotEmpty(tt.value))
		})
	}
}

func TestNotEmptyMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   map[string]int
		wantErr error
		name    string
	}{
		{name: "one entry", value: map[string]int{"a": 1}},
		{name: "empty map", value: map[string]int{}, wantErr: validate.ErrEmpty},
		{name: "nil map", value: nil, wantErr: validate.ErrEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotEmptyMap(tt.value))
		})
	}
}

func TestNotEmptySlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   []int
	}{
		{name: "one element", value: []int{1}},
		{name: "empty slice", value: []int{}, wantErr: validate.ErrEmpty},
		{name: "nil slice", value: nil, wantErr: validate.ErrEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotEmptySlice(tt.value))
		})
	}
}
