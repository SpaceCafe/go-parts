//go:build !with_yaml

package config

import "fmt"

func newYAMLSource(string) (Source, error) {
	return nil, fmt.Errorf("%w: YAML support not yet implemented", ErrInvalidConfig)
}
