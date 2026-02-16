package typeconv_test

import (
	"math"
	"testing"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteSize_Bytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size typeconv.ByteSize
		want uint64
	}{
		{name: "zero", size: 0, want: 0},
		{name: "one byte", size: 1, want: 1},
		{name: "one kilobyte", size: typeconv.KiB, want: 1024},
		{name: "max uint64", size: typeconv.ByteSize(math.MaxUint64), want: math.MaxUint64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.size.Uint64()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestByteSize_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		size typeconv.ByteSize
	}{
		{name: "zero bytes", size: 0, want: "0B"},
		{name: "one byte", size: 1, want: "1B"},
		{name: "kilobyte", size: typeconv.KiB, want: "1.0KiB"},
		{name: "megabyte", size: typeconv.MiB, want: "1.0MiB"},
		{name: "gigabyte", size: typeconv.GiB, want: "1.0GiB"},
		{name: "terabyte", size: typeconv.TiB, want: "1.0TiB"},
		{name: "petabyte", size: typeconv.PiB, want: "1.0PiB"},
		{name: "exabyte", size: typeconv.EiB, want: "1.0EiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.size.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestByteSize_MarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    []byte
		size    typeconv.ByteSize
		wantErr bool
	}{
		{name: "zero bytes", size: 0, want: []byte("0B"), wantErr: false},
		{name: "one gibibyte", size: typeconv.GiB, want: []byte("1.0GiB"), wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.size.MarshalText()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestByteSize_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		want    typeconv.ByteSize
		wantErr bool
	}{
		{name: "valid kilobyte", input: []byte("1KiB"), want: typeconv.KiB, wantErr: false},
		{name: "valid megabyte", input: []byte("1MiB"), want: typeconv.MiB, wantErr: false},
		{name: "zero bytes", input: []byte("0"), want: 0, wantErr: false},
		{name: "invalid text", input: []byte("abcd"), want: 0, wantErr: true},
		{name: "overflow", input: []byte("20000EiB"), want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got typeconv.ByteSize

			err := got.UnmarshalText(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseByteSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    typeconv.ByteSize
		wantErr bool
	}{
		{name: "valid kilobyte", input: "1KiB", want: typeconv.KiB, wantErr: false},
		{name: "valid megabyte", input: "1MiB", want: typeconv.MiB, wantErr: false},
		{name: "valid empty string", input: "", want: 0, wantErr: false},
		{name: "invalid text", input: "abcd", want: 0, wantErr: true},
		{name: "invalid suffix", input: "100F", want: 0, wantErr: true},
		{name: "valid number only", input: "100", want: typeconv.ByteSize(100), wantErr: false},
		{name: "valid underscored", input: "1_024KiB", want: typeconv.MiB, wantErr: false},
		{name: "overflow", input: "20000EiB", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := typeconv.ParseByteSize(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
