package typeconv_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errReaderStringRead = errors.New("read failed")

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errReaderStringRead
}

func TestReaderString_MarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reader  typeconv.ReaderString
		name    string
		want    []byte
		wantErr bool
	}{
		{
			name:    "nil reader",
			reader:  typeconv.ReaderString{Reader: nil},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "empty reader",
			reader:  typeconv.ReaderString{Reader: strings.NewReader("")},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "reader with content",
			reader:  typeconv.ReaderString{Reader: strings.NewReader("hello")},
			want:    []byte("hello"),
			wantErr: false,
		},
		{
			name:    "reader returning error",
			reader:  typeconv.ReaderString{Reader: errReader{}},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.reader.MarshalText()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReaderString_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{name: "nil input", input: nil, want: []byte{}},
		{name: "empty input", input: []byte{}, want: []byte{}},
		{name: "content", input: []byte("hello"), want: []byte("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got typeconv.ReaderString

			err := got.UnmarshalText(tt.input)
			require.NoError(t, err)

			data, readErr := got.MarshalText()
			require.NoError(t, readErr)
			assert.Equal(t, tt.want, data)
		})
	}
}

func TestReaderString_UnmarshalText_SetsBytesReader(t *testing.T) {
	t.Parallel()

	var got typeconv.ReaderString

	err := got.UnmarshalText([]byte("hello"))
	require.NoError(t, err)
	assert.IsType(t, &bytes.Reader{}, got.Reader)
}
