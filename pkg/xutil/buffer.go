package xutil

import (
	"bytes"
	"errors"
	"fmt"
)

var ErrBufferOverflow = errors.New("buffer overflow")

// LimitedBuffer is a buffer that fails once more than limit bytes are written to it. It does not
// embed bytes.Buffer, because io.Copy would then use the promoted ReadFrom and bypass Write.
type LimitedBuffer struct {
	buf      bytes.Buffer
	limit    int64
	exceeded bool
}

// NewLimitedBuffer creates a LimitedBuffer that accepts at most limit bytes.
func NewLimitedBuffer(limit int64) *LimitedBuffer {
	return &LimitedBuffer{limit: limit}
}

// Read reads the buffered data.
func (b *LimitedBuffer) Read(data []byte) (int, error) {
	return b.buf.Read(data)
}

// Write appends data to the buffer, or fails without writing anything once the limit would be
// exceeded.
func (b *LimitedBuffer) Write(data []byte) (int, error) {
	if int64(b.buf.Len())+int64(len(data)) > b.limit {
		b.exceeded = true

		return 0, fmt.Errorf("%w: limit is %d bytes", ErrBufferOverflow, b.limit)
	}

	return b.buf.Write(data)
}
