package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

var _ Source = (*JSONSource)(nil)

// JSONSource loads configuration from a JSON file.
type JSONSource struct {
	Path string

	// AllowUnknownFields accepts keys that no field of the target matches. By default they fail
	// loading, so a misspelled key surfaces instead of being ignored. Enable it for files shared
	// with other programs.
	AllowUnknownFields bool
}

func (JSONSource) GenerateTemplate(target any, output io.Writer) error {
	return json.NewEncoder(output).Encode(target)
}

func (s JSONSource) Load(target any) error {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return fmt.Errorf("%w: read JSON file: %w", ErrConfigNotFound, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if !s.AllowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	err = decoder.Decode(target)
	if err != nil {
		return fmt.Errorf("%w: unmarshal JSON: %w", ErrInvalidConfig, err)
	}

	// A second value, or anything but whitespace, after the first one is trailing data.
	_, err = decoder.Token()
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: unexpected data after JSON value", ErrInvalidConfig)
		}

		return fmt.Errorf("%w: unmarshal JSON: %w", ErrInvalidConfig, err)
	}

	return nil
}
