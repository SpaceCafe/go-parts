package xutil_test

import (
	"io"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/xutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitedBuffer_Write(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    string
		writes  []string
		limit   int64
		wantErr bool
	}{
		{name: "below limit", limit: 10, writes: []string{"hello"}, want: "hello"},
		{name: "exactly at limit", limit: 5, writes: []string{"hello"}, want: "hello"},
		{name: "above limit", limit: 4, writes: []string{"hello"}, want: "", wantErr: true},
		{
			name:   "cumulative writes at limit",
			limit:  10,
			writes: []string{"hello", "world"},
			want:   "helloworld",
		},
		{
			name:    "cumulative writes above limit",
			limit:   9,
			writes:  []string{"hello", "world"},
			want:    "hello",
			wantErr: true,
		},
		{name: "empty write with zero limit", limit: 0, writes: []string{""}, want: ""},
		{name: "write with zero limit", limit: 0, writes: []string{"a"}, want: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := xutil.NewLimitedBuffer(tt.limit)

			for _, w := range tt.writes[:len(tt.writes)-1] {
				_, err := buf.Write([]byte(w))
				require.NoError(t, err)
			}

			last := tt.writes[len(tt.writes)-1]
			written, err := buf.Write([]byte(last))

			if tt.wantErr {
				require.ErrorIs(t, err, xutil.ErrBufferOverflow)
				assert.Zero(t, written)
			} else {
				require.NoError(t, err)
				assert.Equal(t, len(last), written)
			}

			data, err := io.ReadAll(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(data))
			assert.Equal(t, tt.wantErr, buf.Exceeded())
		})
	}
}

func TestLimitedBuffer_WriteErrorMentionsLimit(t *testing.T) {
	t.Parallel()

	buf := xutil.NewLimitedBuffer(3)

	_, err := buf.Write([]byte("hello"))
	require.ErrorIs(t, err, xutil.ErrBufferOverflow)
	assert.Contains(t, err.Error(), "limit is 3 bytes")
}

func TestLimitedBuffer_Read(t *testing.T) {
	t.Parallel()

	buf := xutil.NewLimitedBuffer(10)

	_, err := buf.Write([]byte("hello"))
	require.NoError(t, err)

	data, err := io.ReadAll(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestLimitedBuffer_Copy(t *testing.T) {
	t.Parallel()

	// LimitedBuffer must not implement io.ReaderFrom, otherwise io.Copy bypasses Write.
	_, ok := any(xutil.NewLimitedBuffer(0)).(io.ReaderFrom)
	require.False(t, ok)

	tests := []struct {
		name    string
		src     string
		limit   int64
		wantErr bool
	}{
		{name: "source fits", src: "hello", limit: 5},
		{name: "source too large", src: "hello world", limit: 5, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := xutil.NewLimitedBuffer(tt.limit)

			// Hide strings.Reader.WriteTo so io.Copy runs its own read/write loop.
			_, err := io.Copy(buf, struct{ io.Reader }{strings.NewReader(tt.src)})

			data, readErr := io.ReadAll(buf)
			require.NoError(t, readErr)
			assert.Equal(t, tt.wantErr, buf.Exceeded())

			if tt.wantErr {
				require.ErrorIs(t, err, xutil.ErrBufferOverflow)
				assert.LessOrEqual(t, int64(len(data)), tt.limit)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.src, string(data))
		})
	}
}

func TestLimitedBuffer_WriteTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		want         string
		writes       []string
		limit        int64
		wantExceeded bool
	}{
		{name: "below limit", limit: 10, writes: []string{"hello"}, want: "hello"},
		{name: "exactly at limit", limit: 5, writes: []string{"hello"}, want: "hello"},
		{
			name:         "above limit",
			limit:        4,
			writes:       []string{"hello"},
			want:         "hell",
			wantExceeded: true,
		},
		{
			name:         "cumulative writes above limit",
			limit:        7,
			writes:       []string{"hello", "world", "again"},
			want:         "hellowo",
			wantExceeded: true,
		},
		{
			name:         "write with zero limit",
			limit:        0,
			writes:       []string{"a"},
			want:         "",
			wantExceeded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := xutil.NewLimitedBuffer(tt.limit, xutil.WithTruncate())

			for _, w := range tt.writes {
				written, err := buf.Write([]byte(w))
				require.NoError(t, err)
				assert.Equal(t, len(w), written)
			}

			assert.Equal(t, tt.want, buf.String())
			assert.Equal(t, tt.wantExceeded, buf.Exceeded())
		})
	}
}

func TestLimitedBuffer_CopyTruncate(t *testing.T) {
	t.Parallel()

	buf := xutil.NewLimitedBuffer(5, xutil.WithTruncate())

	// Hide strings.Reader.WriteTo so io.Copy runs its own read/write loop.
	written, err := io.Copy(buf, struct{ io.Reader }{strings.NewReader("hello world")})
	require.NoError(t, err)
	assert.Equal(t, int64(len("hello world")), written)
	assert.Equal(t, "hello", buf.String())
	assert.True(t, buf.Exceeded())
}

func TestLimitedBuffer_String(t *testing.T) {
	t.Parallel()

	buf := xutil.NewLimitedBuffer(10)

	_, err := buf.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", buf.String())
}
