//go:build with_yaml

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

var _ Source = (*YAMLSource)(nil)

// yamlSupported reports whether YAML support is compiled in, so configPaths includes YAML files.
const yamlSupported = true

// YAMLSource loads configuration from a YAML file.
type YAMLSource struct {
	Path string

	// AllowUnknownFields accepts keys that no field of the target matches. By default they fail
	// loading, so a misspelled key surfaces instead of being ignored. Enable it for files shared
	// with other programs.
	AllowUnknownFields bool
}

func (YAMLSource) GenerateTemplate(target any, output io.Writer) error {
	return yaml.NewEncoder(output).Encode(target)
}

func (s YAMLSource) Load(target any) error {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return fmt.Errorf("%w: read YAML file: %w", ErrConfigNotFound, err)
	}

	var opts []yaml.DecodeOption
	if !s.AllowUnknownFields {
		opts = append(opts, yaml.DisallowUnknownField())
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data), opts...)

	err = decoder.Decode(target)
	if err != nil {
		return fmt.Errorf("%w: unmarshal YAML: %w", ErrInvalidConfig, err)
	}

	return checkSingleYAMLDocument(decoder)
}

// checkSingleYAMLDocument rejects a document with content after the first one. A config file holds
// one document: a second one does not override the first, so loading only the first would
// silently drop whatever the second was meant to change. An empty document, such as the one after
// a trailing "---", is allowed.
func checkSingleYAMLDocument(decoder *yaml.Decoder) error {
	for {
		var extra any

		err := decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w: unmarshal YAML: %w", ErrInvalidConfig, err)
		}

		if extra != nil {
			return fmt.Errorf(
				"%w: multiple YAML documents, a config file holds one",
				ErrInvalidConfig,
			)
		}
	}
}

func newYAMLSource(filename string, allowUnknownFields bool) (*YAMLSource, error) {
	return &YAMLSource{Path: filename, AllowUnknownFields: allowUnknownFields}, nil
}
