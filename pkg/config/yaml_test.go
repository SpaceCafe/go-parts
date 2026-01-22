//go:build with_yaml

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestYAMLSource_Load(t *testing.T) {
	t.Parallel()

	// Create test files.
	validFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(validFile, []byte("name: \"test-app\"\nport: 8080"), 0o600)
	require.NoError(t, err)

	invalidFile := filepath.Join(t.TempDir(), "invalid.yaml")
	err = os.WriteFile(invalidFile, []byte(`invalid yaml`), 0o600)
	require.NoError(t, err)

	testFileSourceLoad(t, func(path string) config.Source {
		return config.YAMLSource{
			Path: path,
		}
	}, validFile, invalidFile)
}
