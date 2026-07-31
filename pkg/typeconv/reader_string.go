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
// It reads all data from the underlying io.Reader and returns it as text.
func (s ReaderString) MarshalText() ([]byte, error) {
	if s.Reader == nil {
		return []byte{}, nil
	}

	data, err := io.ReadAll(s.Reader)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It sets the underlying io.Reader to a new bytes.Reader with the provided data.
func (s *ReaderString) UnmarshalText(data []byte) error {
	s.Reader = bytes.NewReader(data)

	return nil
}
