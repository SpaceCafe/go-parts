package xutil

import (
	"bytes"
	"errors"
	"fmt"
)

var ErrBufferOverflow = errors.New("buffer overflow")

// LimitedBuffer is a buffer that holds at most limit bytes. By default, a write that would exceed
// the limit fails; with WithTruncate, the excess is dropped instead. It does not embed
// bytes.Buffer, because io.Copy would then use the promoted ReadFrom and bypass Write.
type LimitedBuffer struct {
	buf      bytes.Buffer
	limit    int64
	exceeded bool
	truncate bool
}

// LimitedBufferOption is a functional option for configuring a LimitedBuffer.
type LimitedBufferOption func(*LimitedBuffer)

// WithTruncate makes Write keep the part of the data that fits into the limit and silently drop
// the rest, instead of failing. Use it for a writer whose failure would harm the producer, such as
// the stderr of a process, which a failed write closes and so kills with SIGPIPE.
func WithTruncate() LimitedBufferOption {
	return func(buf *LimitedBuffer) {
		buf.truncate = true
	}
}

// NewLimitedBuffer creates a LimitedBuffer that accepts at most limit bytes.
func NewLimitedBuffer(limit int64, opts ...LimitedBufferOption) *LimitedBuffer {
	obj := &LimitedBuffer{limit: limit}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// Exceeded reports whether a write was rejected, or with WithTruncate cut short, because it would
// have exceeded the limit.
func (b *LimitedBuffer) Exceeded() bool {
	return b.exceeded
}

// Read reads the buffered data.
func (b *LimitedBuffer) Read(data []byte) (int, error) {
	return b.buf.Read(data)
}

// String returns the unread data as a string.
func (b *LimitedBuffer) String() string {
	return b.buf.String()
}

// Write appends data to the buffer. Once the limit would be exceeded, it fails without writing
// anything, or with WithTruncate writes the part that fits and reports all of data as written.
func (b *LimitedBuffer) Write(data []byte) (int, error) {
	room := b.limit - int64(b.buf.Len())
	if int64(len(data)) <= room {
		return b.buf.Write(data)
	}

	b.exceeded = true

	if !b.truncate {
		return 0, fmt.Errorf("%w: limit is %d bytes", ErrBufferOverflow, b.limit)
	}

	if room > 0 {
		_, _ = b.buf.Write(data[:room])
	}

	return len(data), nil
}
