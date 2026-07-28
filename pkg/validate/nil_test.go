package validate_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
)

func TestNilMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   map[string]int
		wantErr error
		name    string
	}{
		{name: "nil map", value: nil},
		{name: "empty map", value: map[string]int{}, wantErr: validate.ErrNotNil},
		{name: "one entry", value: map[string]int{"a": 1}, wantErr: validate.ErrNotNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NilMap(tt.value))
		})
	}
}

func TestNilPointer(t *testing.T) {
	t.Parallel()

	value := 1

	tests := []struct {
		value   *int
		wantErr error
		name    string
	}{
		{name: "nil pointer", value: nil},
		{name: "pointer to value", value: &value, wantErr: validate.ErrNotNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NilPointer(tt.value))
		})
	}
}

func TestNilSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   []int
	}{
		{name: "nil slice", value: nil},
		{name: "empty slice", value: []int{}, wantErr: validate.ErrNotNil},
		{name: "one element", value: []int{1}, wantErr: validate.ErrNotNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NilSlice(tt.value))
		})
	}
}

func TestNotNilMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   map[string]int
		wantErr error
		name    string
	}{
		{name: "empty map", value: map[string]int{}},
		{name: "one entry", value: map[string]int{"a": 1}},
		{name: "nil map", value: nil, wantErr: validate.ErrNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotNilMap(tt.value))
		})
	}
}

func TestNotNilPointer(t *testing.T) {
	t.Parallel()

	value := 1

	tests := []struct {
		value   *int
		wantErr error
		name    string
	}{
		{name: "pointer to value", value: &value},
		{name: "nil pointer", value: nil, wantErr: validate.ErrNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotNilPointer(tt.value))
		})
	}
}

func TestNotNilSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   []int
	}{
		{name: "empty slice", value: []int{}},
		{name: "one element", value: []int{1}},
		{name: "nil slice", value: nil, wantErr: validate.ErrNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NotNilSlice(tt.value))
		})
	}
}
