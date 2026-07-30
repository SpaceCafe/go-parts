package xutil_test

import (
	"errors"
	"testing"

	"github.com/spacecafe/go-parts/pkg/xutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errMock = errors.New("mock error")

func TestMust(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err       error
		name      string
		value     int
		wantPanic bool
	}{
		{name: "no error", value: 42},
		{name: "with error", value: 42, err: errMock, wantPanic: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				require.Panics(t, func() {
					xutil.Must(tt.value, tt.err)
				})

				return
			}

			assert.Equal(t, tt.value, xutil.Must(tt.value, tt.err))
		})
	}
}

func TestClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		lowerBoundary int
		value         int
		upperBoundary int
		want          int
	}{
		{name: "inside range", lowerBoundary: 1, value: 5, upperBoundary: 10, want: 5},
		{name: "below range", lowerBoundary: 1, value: 0, upperBoundary: 10, want: 1},
		{name: "above range", lowerBoundary: 1, value: 11, upperBoundary: 10, want: 10},
		{name: "on lower bound", lowerBoundary: 1, value: 1, upperBoundary: 10, want: 1},
		{name: "on upper bound", lowerBoundary: 1, value: 10, upperBoundary: 10, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, xutil.Clamp(tt.lowerBoundary, tt.value, tt.upperBoundary))
		})
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		value        string
		defaultValue string
		want         string
	}{
		{name: "non-zero value", value: "set", defaultValue: "fallback", want: "set"},
		{name: "zero value", value: "", defaultValue: "fallback", want: "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, xutil.Default(tt.value, tt.defaultValue))
		})
	}
}

func TestDefaultSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		value        []int
		defaultValue []int
		want         []int
	}{
		{name: "non-empty value", value: []int{1, 2}, defaultValue: []int{9}, want: []int{1, 2}},
		{name: "empty value", value: []int{}, defaultValue: []int{9}, want: []int{9}},
		{name: "nil value", value: nil, defaultValue: []int{9}, want: []int{9}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, xutil.DefaultSlice(tt.value, tt.defaultValue))
		})
	}
}

func TestDefaultMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value        map[string]int
		defaultValue map[string]int
		want         map[string]int
		name         string
	}{
		{
			name:         "non-empty value",
			value:        map[string]int{"a": 1},
			defaultValue: map[string]int{"b": 2},
			want:         map[string]int{"a": 1},
		},
		{
			name:         "empty value",
			value:        map[string]int{},
			defaultValue: map[string]int{"b": 2},
			want:         map[string]int{"b": 2},
		},
		{
			name:         "nil value",
			value:        nil,
			defaultValue: map[string]int{"b": 2},
			want:         map[string]int{"b": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, xutil.DefaultMap(tt.value, tt.defaultValue))
		})
	}
}
