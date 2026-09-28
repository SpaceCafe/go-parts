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

func TestJSONSource_Load_UnknownFields(t *testing.T) {
	t.Parallel()

	testUnknownFields(t, "config.json", `{"name": "app", "prot": 8080}`,
		func(path string, allow bool) config.Source {
			return config.JSONSource{Path: path, AllowUnknownFields: allow}
		})
}

// testUnknownFields checks that the source built by newSource rejects the misspelled key "prot" in
// content by default and accepts it with allow set.
func testUnknownFields(
	t *testing.T,
	filename, content string,
	newSource func(path string, allow bool) config.Source,
) {
	t.Helper()

	path := filepath.Join(t.TempDir(), filename)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	var strict MockConfig

	err := newSource(path, false).Load(&strict)
	require.ErrorIs(t, err, config.ErrInvalidConfig)
	require.ErrorContains(t, err, "prot")

	var lenient MockConfig

	require.NoError(t, newSource(path, true).Load(&lenient))
	require.Equal(t, "app", lenient.Name)
}

func TestJSONSource_Load_TrailingData(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"garbage after the object": `{"name": "app", "port": 8080} garbage`,
		"second object":            `{"name": "app", "port": 8080} {"port": 9090}`,
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			var target MockConfig

			require.ErrorIs(t, config.JSONSource{Path: path}.Load(&target), config.ErrInvalidConfig)
		})
	}
}
