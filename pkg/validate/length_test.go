package validate_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
)

func TestLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
		target  int
	}{
		{name: "matching length", value: "abc", target: 3},
		{name: "empty value", value: "", target: 0},
		{name: "too short", value: "ab", target: 3, wantErr: validate.ErrLength},
		{name: "too long", value: "abcd", target: 3, wantErr: validate.ErrLength},
		{name: "multi byte runes", value: "café", target: 5},
		{
			name:    "multi byte runes counted as runes",
			value:   "café",
			target:  4,
			wantErr: validate.ErrLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Length[string](tt.target)(tt.value))
		})
	}
}

func TestLengthBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      string
		lowerBound int
		upperBound int
	}{
		{name: "inside range", value: "abc", lowerBound: 2, upperBound: 4},
		{name: "on lower bound", value: "ab", lowerBound: 2, upperBound: 4},
		{name: "on upper bound", value: "abcd", lowerBound: 2, upperBound: 4},
		{
			name:       "too short",
			value:      "a",
			lowerBound: 2,
			upperBound: 4,
			wantErr:    validate.ErrLengthBetween,
		},
		{
			name:       "too long",
			value:      "abcde",
			lowerBound: 2,
			upperBound: 4,
			wantErr:    validate.ErrLengthBetween,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(
				t,
				tt.wantErr,
				validate.LengthBetween[string](tt.lowerBound, tt.upperBound)(tt.value),
			)
		})
	}
}

func TestLengthMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      string
		upperBound int
	}{
		{name: "below bound", value: "ab", upperBound: 3},
		{name: "on bound", value: "abc", upperBound: 3},
		{name: "empty value", value: "", upperBound: 3},
		{name: "above bound", value: "abcd", upperBound: 3, wantErr: validate.ErrLengthMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.LengthMax[string](tt.upperBound)(tt.value))
		})
	}
}

func TestLengthMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      string
		lowerBound int
	}{
		{name: "above bound", value: "abcd", lowerBound: 3},
		{name: "on bound", value: "abc", lowerBound: 3},
		{name: "below bound", value: "ab", lowerBound: 3, wantErr: validate.ErrLengthMin},
		{name: "empty value", value: "", lowerBound: 1, wantErr: validate.ErrLengthMin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.LengthMin[string](tt.lowerBound)(tt.value))
		})
	}
}

func TestMapLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   map[string]int
		wantErr error
		name    string
		target  int
	}{
		{name: "matching length", value: map[string]int{"a": 1, "b": 2}, target: 2},
		{name: "empty map", value: map[string]int{}, target: 0},
		{name: "nil map", value: nil, target: 0},
		{
			name:    "too few entries",
			value:   map[string]int{"a": 1},
			target:  2,
			wantErr: validate.ErrLength,
		},
		{
			name:    "too many entries",
			value:   map[string]int{"a": 1, "b": 2, "c": 3},
			target:  2,
			wantErr: validate.ErrLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.MapLength[string, int](tt.target)(tt.value))
		})
	}
}

func TestMapLengthBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value      map[string]int
		wantErr    error
		name       string
		lowerBound int
		upperBound int
	}{
		{name: "inside range", value: map[string]int{"a": 1, "b": 2}, lowerBound: 1, upperBound: 3},
		{name: "on lower bound", value: map[string]int{"a": 1}, lowerBound: 1, upperBound: 3},
		{
			name:       "below range",
			value:      nil,
			lowerBound: 1,
			upperBound: 3,
			wantErr:    validate.ErrLengthBetween,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(
				t,
				tt.wantErr,
				validate.MapLengthBetween[string, int](tt.lowerBound, tt.upperBound)(tt.value),
			)
		})
	}
}

func TestMapLengthMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value      map[string]int
		wantErr    error
		name       string
		upperBound int
	}{
		{name: "below bound", value: map[string]int{"a": 1}, upperBound: 2},
		{name: "on bound", value: map[string]int{"a": 1, "b": 2}, upperBound: 2},
		{
			name:       "above bound",
			value:      map[string]int{"a": 1, "b": 2, "c": 3},
			upperBound: 2,
			wantErr:    validate.ErrLengthMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.MapLengthMax[string, int](tt.upperBound)(tt.value))
		})
	}
}

func TestMapLengthMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value      map[string]int
		wantErr    error
		name       string
		lowerBound int
	}{
		{name: "above bound", value: map[string]int{"a": 1, "b": 2}, lowerBound: 1},
		{name: "on bound", value: map[string]int{"a": 1}, lowerBound: 1},
		{name: "nil map", value: nil, lowerBound: 1, wantErr: validate.ErrLengthMin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.MapLengthMin[string, int](tt.lowerBound)(tt.value))
		})
	}
}

func TestSliceLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   []int
		target  int
	}{
		{name: "matching length", value: []int{1, 2, 3}, target: 3},
		{name: "empty slice", value: []int{}, target: 0},
		{name: "nil slice", value: nil, target: 0},
		{name: "too few elements", value: []int{1}, target: 3, wantErr: validate.ErrLength},
		{
			name:    "too many elements",
			value:   []int{1, 2, 3, 4},
			target:  3,
			wantErr: validate.ErrLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.SliceLength[int](tt.target)(tt.value))
		})
	}
}

func TestSliceLengthBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      []int
		lowerBound int
		upperBound int
	}{
		{name: "inside range", value: []int{1, 2}, lowerBound: 1, upperBound: 3},
		{name: "on upper bound", value: []int{1, 2, 3}, lowerBound: 1, upperBound: 3},
		{
			name:       "above range",
			value:      []int{1, 2, 3, 4},
			lowerBound: 1,
			upperBound: 3,
			wantErr:    validate.ErrLengthBetween,
		},
		{
			name:       "nil slice",
			value:      nil,
			lowerBound: 1,
			upperBound: 3,
			wantErr:    validate.ErrLengthBetween,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(
				t,
				tt.wantErr,
				validate.SliceLengthBetween[int](tt.lowerBound, tt.upperBound)(tt.value),
			)
		})
	}
}

func TestSliceLengthMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      []int
		upperBound int
	}{
		{name: "below bound", value: []int{1}, upperBound: 2},
		{name: "on bound", value: []int{1, 2}, upperBound: 2},
		{name: "above bound", value: []int{1, 2, 3}, upperBound: 2, wantErr: validate.ErrLengthMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.SliceLengthMax[int](tt.upperBound)(tt.value))
		})
	}
}

func TestSliceLengthMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		value      []int
		lowerBound int
	}{
		{name: "above bound", value: []int{1, 2}, lowerBound: 1},
		{name: "on bound", value: []int{1}, lowerBound: 1},
		{name: "empty slice", value: []int{}, lowerBound: 1, wantErr: validate.ErrLengthMin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.SliceLengthMin[int](tt.lowerBound)(tt.value))
		})
	}
}
