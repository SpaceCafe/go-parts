//go:build !with_yaml

package config

import "fmt"

// yamlSupported reports whether YAML support is compiled in. Without it, configPaths skips YAML
// files, so a YAML file written for another tool does not abort AutoLoad.
const yamlSupported = false

func newYAMLSource(string, bool) (Source, error) {
	return nil, fmt.Errorf("%w: YAML support requires the with_yaml build tag", ErrInvalidConfig)
}
