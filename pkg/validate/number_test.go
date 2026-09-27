package validate_test

import (
	"math"
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/require"
)

func TestBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		lowerBound int
		upperBound int
		value      int
	}{
		{name: "inside range", lowerBound: 1, upperBound: 10, value: 5},
		{name: "on lower bound", lowerBound: 1, upperBound: 10, value: 1},
		{name: "on upper bound", lowerBound: 1, upperBound: 10, value: 10},
		{
			name:       "below range",
			lowerBound: 1,
			upperBound: 10,
			value:      0,
			wantErr:    validate.ErrNotBetween,
		},
		{
			name:       "above range",
			lowerBound: 1,
			upperBound: 10,
			value:      11,
			wantErr:    validate.ErrNotBetween,
		},
		{name: "negative range", lowerBound: -10, upperBound: -1, value: -5},
		{
			name:       "inverted range",
			lowerBound: 10,
			upperBound: 1,
			value:      5,
			wantErr:    validate.ErrNotBetween,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Between(tt.lowerBound, tt.upperBound)(tt.value))
		})
	}
}

func TestBetweenFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		lowerBound float64
		upperBound float64
		value      float64
	}{
		{name: "inside range", lowerBound: 0.5, upperBound: 1.5, value: 1},
		{
			name:       "just below range",
			lowerBound: 0.5,
			upperBound: 1.5,
			value:      0.4999,
			wantErr:    validate.ErrNotBetween,
		},
		{
			name:       "just above range",
			lowerBound: 0.5,
			upperBound: 1.5,
			value:      1.5001,
			wantErr:    validate.ErrNotBetween,
		},
		{
			name:       "not a number",
			lowerBound: 0.5,
			upperBound: 1.5,
			value:      math.NaN(),
			wantErr:    validate.ErrNaN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Between(tt.lowerBound, tt.upperBound)(tt.value))
		})
	}
}

func TestMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		upperBound int
		value      int
	}{
		{name: "below bound", upperBound: 10, value: 9},
		{name: "on bound", upperBound: 10, value: 10},
		{name: "above bound", upperBound: 10, value: 11, wantErr: validate.ErrMax},
		{name: "negative bound", upperBound: -1, value: -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Max(tt.upperBound)(tt.value))
		})
	}
}

func TestMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		lowerBound int
		value      int
	}{
		{name: "above bound", lowerBound: 10, value: 11},
		{name: "on bound", lowerBound: 10, value: 10},
		{name: "below bound", lowerBound: 10, value: 9, wantErr: validate.ErrMin},
		{name: "negative bound", lowerBound: -10, value: -9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Min(tt.lowerBound)(tt.value))
		})
	}
}

func TestMultipleOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		divisor int
		value   int
	}{
		{name: "exact multiple", divisor: 3, value: 9},
		{name: "zero is a multiple of everything", divisor: 3, value: 0},
		{name: "not a multiple", divisor: 3, value: 10, wantErr: validate.ErrNotMultipleOf},
		{name: "negative value", divisor: 3, value: -9},
		{name: "negative divisor", divisor: -3, value: 9},
		{name: "divisor of one", divisor: 1, value: 7},
		{name: "divisor of zero", divisor: 0, value: 9, wantErr: validate.ErrDivisorZero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.MultipleOf(tt.divisor)(tt.value))
		})
	}
}

func TestNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   int
	}{
		{name: "negative", value: -1},
		{name: "zero", value: 0, wantErr: validate.ErrNotNegative},
		{name: "positive", value: 1, wantErr: validate.ErrNotNegative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Negative(tt.value))
		})
	}
}

func TestNonNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   int
	}{
		{name: "positive", value: 1},
		{name: "zero", value: 0},
		{name: "negative", value: -1, wantErr: validate.ErrNotNonNegative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NonNegative(tt.value))
		})
	}
}

func TestPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   int
	}{
		{name: "wildcard port", value: 0},
		{name: "well known port", value: 80},
		{name: "highest port", value: math.MaxUint16},
		{name: "above highest port", value: math.MaxUint16 + 1, wantErr: validate.ErrNotBetween},
		{name: "negative port", value: -1, wantErr: validate.ErrNotBetween},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Port(tt.value))
		})
	}
}

func TestPositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   int
	}{
		{name: "positive", value: 1},
		{name: "zero", value: 0, wantErr: validate.ErrNotPositive},
		{name: "negative", value: -1, wantErr: validate.ErrNotPositive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Positive(tt.value))
		})
	}
}

func TestNaN(t *testing.T) {
	t.Parallel()

	nan := math.NaN()

	validators := map[string]func(float64) error{
		"Between":     validate.Between(0.0, 1.0),
		"Max":         validate.Max(1.0),
		"Min":         validate.Min(0.0),
		"Negative":    validate.Negative[float64],
		"NonNegative": validate.NonNegative[float64],
		"Port":        validate.Port[float64],
		"Positive":    validate.Positive[float64],
	}

	for name, validator := range validators {
		require.ErrorIs(t, validator(nan), validate.ErrNaN, name)
	}

	require.NoError(t, validate.Positive(1.5), "a regular float still passes")
	require.NoError(t, validate.Positive(3), "integers are never NaN")
}
