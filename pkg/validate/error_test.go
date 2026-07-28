package validate_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
)

var errSentinel = errors.New("sentinel")

func TestError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   error
		wantErr error
		name    string
	}{
		{name: "plain error", value: errSentinel},
		{name: "wrapped error", value: fmt.Errorf("context: %w", errSentinel)},
		{name: "no error", value: nil, wantErr: validate.ErrError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.Error(tt.value))
		})
	}
}

func TestNoError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   error
		wantErr error
		name    string
	}{
		{name: "no error", value: nil},
		{name: "plain error", value: errSentinel, wantErr: validate.ErrNoError},
		{
			name:    "wrapped error",
			value:   fmt.Errorf("context: %w", errSentinel),
			wantErr: validate.ErrNoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireErr(t, tt.wantErr, validate.NoError(tt.value))
		})
	}
}
