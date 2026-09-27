package typeconv

import (
	"bytes"
	"io"
)

// ReaderString wraps an io.Reader and marshals its content as a JSON string.
type ReaderString struct {
	io.Reader
}

// MarshalText implements encoding.TextMarshaler.
// It reads all remaining data from the underlying io.Reader, without a size limit, and returns it
// as text. If the reader is an io.Seeker (as the one set by UnmarshalText is), it is rewound to its
// previous position afterwards, so marshaling can be repeated. Any other reader is consumed: a
// second MarshalText returns empty text.
func (s ReaderString) MarshalText() ([]byte, error) {
	if s.Reader == nil {
		return []byte{}, nil
	}

	seeker, canSeek := s.Reader.(io.Seeker)

	var start int64

	if canSeek {
		var err error

		start, err = seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
	}

	data, err := io.ReadAll(s.Reader)
	if err != nil {
		return nil, err
	}

	if canSeek {
		_, err = seeker.Seek(start, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It sets the underlying io.Reader to a new bytes.Reader with the provided data.
func (s *ReaderString) UnmarshalText(data []byte) error {
	s.Reader = bytes.NewReader(data)

	return nil
}
