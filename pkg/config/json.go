package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var _ Source = (*JSONSource)(nil)

// JSONSource loads configuration from a JSON file.
type JSONSource struct {
	Path string
}

func (s JSONSource) Load(target any) error {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return fmt.Errorf("%w: read JSON file: %w", ErrConfigNotFound, err)
	}

	err = json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("%w: unmarshal JSON: %w", ErrInvalidConfig, err)
	}

	return nil
}
