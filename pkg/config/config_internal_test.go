package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Changes the working directory.
func TestFindConfigSource(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile("config.json", []byte(`{}`), 0o600))

	t.Run("explicit path that does not exist", func(t *testing.T) {
		_, err := findConfigSource("app", filepath.Join(t.TempDir(), "missing.json"), false)
		require.ErrorIs(t, err, ErrConfigNotFound)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("explicit path that exists", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "explicit.json")
		require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))

		source, err := findConfigSource("app", path, false)
		require.NoError(t, err)
		assert.Equal(t, &JSONSource{Path: path}, source)
	})

	t.Run("search list without explicit path", func(t *testing.T) {
		source, err := findConfigSource("app", "", false)
		require.NoError(t, err)
		assert.Equal(t, &JSONSource{Path: "config.json"}, source)
	})

	t.Run("unreadable directory in the search list", func(t *testing.T) {
		if os.Geteuid() == 0 || runtime.GOOS == "windows" {
			t.Skip("directory permissions are not enforced")
		}

		require.NoError(t, os.Mkdir("config", 0o000))
		t.Cleanup(func() { _ = os.Chmod("config", 0o700) })
		require.NoError(t, os.Remove("config.json"))

		_, err := findConfigSource("app", "", false)
		require.ErrorIs(t, err, ErrConfigNotFound)
		require.ErrorIs(t, err, fs.ErrPermission)
	})
}

func TestParseAutoLoadArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		args    []string
		want    autoLoadArgs
	}{
		{name: "no args"},
		{
			name: "config with space and app flags",
			args: []string{"-port", "9000", "--config", "prod.json", "-v"},
			want: autoLoadArgs{configPath: "prod.json"},
		},
		{
			name: "config with equals",
			args: []string{"-config=prod.json"},
			want: autoLoadArgs{configPath: "prod.json"},
		},
		{name: "config without value", args: []string{"--config"}, wantErr: ErrInvalidArgs},
		{
			name: "generate template with default path",
			args: []string{"--generate-template", "--config", "prod.json"},
			want: autoLoadArgs{
				configPath:       "prod.json",
				generateTemplate: true,
				templatePath:     defaultTemplatePath,
			},
		},
		{
			name: "generate template with path",
			args: []string{"-generate-template=out.yaml"},
			want: autoLoadArgs{generateTemplate: true, templatePath: "out.yaml"},
		},
		{name: "generate template false", args: []string{"-generate-template=false"}},
		{name: "stops at double dash", args: []string{"--", "--config", "x.json"}},
		{name: "positional argument is ignored", args: []string{"config"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAutoLoadArgs(tt.args)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateEnvName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"CCBin":     "CC_BIN",
		"HTTPPort":  "HTTP_PORT",
		"maxSize":   "MAX_SIZE",
		"ÄÄÄÄB":     "ÄÄÄÄB",
		"GrößeMax":  "GRÖßE_MAX",
		"ÜberCache": "ÜBER_CACHE",
	}

	for fieldName, want := range tests {
		assert.Equal(t, "P_"+want, createEnvName("P", fieldName, ""), fieldName)
	}
}

//nolint:paralleltest // Changes the working directory.
func TestFindConfigSource_YAMLBuildTag(t *testing.T) {
	t.Chdir(t.TempDir())

	// A YAML file ahead of the JSON file in the search list, for example one written for another tool.
	require.NoError(t, os.WriteFile("app.yaml", []byte("name: app\n"), 0o600))
	require.NoError(t, os.WriteFile("config.json", []byte(`{}`), 0o600))

	source, err := findConfigSource("app", "", false)
	require.NoError(t, err)

	if yamlSupported {
		assert.NotEqual(t, &JSONSource{Path: "config.json"}, source)
	} else {
		assert.Equal(t, &JSONSource{Path: "config.json"}, source)

		_, err = findConfigSource("app", "app.yaml", false)
		require.ErrorContains(t, err, "with_yaml")
	}
}
