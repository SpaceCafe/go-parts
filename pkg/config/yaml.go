//go:build with_yaml

package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

var _ Source = (*YAMLSource)(nil)

// YAMLSource loads configuration from a YAML file.
type YAMLSource struct {
	Path string
}

func (s YAMLSource) Load(target any) error {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return fmt.Errorf("%w: read YAML file: %w", ErrConfigNotFound, err)
	}

	err = yaml.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("%w: unmarshal YAML: %w", ErrInvalidConfig, err)
	}

	return nil
}
