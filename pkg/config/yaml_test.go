//go:build with_yaml

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/stretchr/testify/assert"
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

func TestYAMLSource_Load_UnknownFields(t *testing.T) {
	t.Parallel()

	testUnknownFields(t, "config.yaml", "name: app\nprot: 8080\n",
		func(path string, allow bool) config.Source {
			return config.YAMLSource{Path: path, AllowUnknownFields: allow}
		})
}

func TestYAMLSource_Load_MultipleDocuments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		content string
		wantErr bool
	}{
		"single document":    {content: "name: app\nport: 8080\n"},
		"leading separator":  {content: "---\nname: app\nport: 8080\n"},
		"trailing separator": {content: "name: app\nport: 8080\n---\n"},
		"second document": {
			content: "name: app\nport: 8080\n---\nport: 9090\n",
			wantErr: true,
		},
		"second invalid document": {
			content: "name: app\nport: 8080\n---\n{{not yaml\n",
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			var target MockConfig

			err := config.YAMLSource{Path: path}.Load(&target)
			if tt.wantErr {
				require.ErrorIs(t, err, config.ErrInvalidConfig)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, 8080, target.Port)
		})
	}
}
