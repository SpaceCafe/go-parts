package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestJSONSource_Load(t *testing.T) {
	t.Parallel()

	// Create test files.
	validFile := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(validFile, []byte(`{"name": "test-app", "port": 8080}`), 0o600)
	require.NoError(t, err)

	invalidFile := filepath.Join(t.TempDir(), "invalid.json")
	err = os.WriteFile(invalidFile, []byte(`{invalid json}`), 0o600)
	require.NoError(t, err)

	testFileSourceLoad(t, func(path string) config.Source {
		return config.JSONSource{
			Path: path,
		}
	}, validFile, invalidFile)
}
