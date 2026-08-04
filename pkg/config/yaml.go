//go:build with_yaml

package config

import (
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

var _ Source = (*YAMLSource)(nil)

// YAMLSource loads configuration from a YAML file.
type YAMLSource struct {
	Path string
}

func (YAMLSource) GenerateTemplate(target any, output io.Writer) error {
	return yaml.NewEncoder(output).Encode(target)
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

func newYAMLSource(filename string) (*YAMLSource, error) {
	return &YAMLSource{Path: filename}, nil
}
