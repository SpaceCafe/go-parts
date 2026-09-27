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
		_, err := findConfigSource("app", filepath.Join(t.TempDir(), "missing.json"))
		require.ErrorIs(t, err, ErrConfigNotFound)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("explicit path that exists", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "explicit.json")
		require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))

		source, err := findConfigSource("app", path)
		require.NoError(t, err)
		assert.Equal(t, &JSONSource{Path: path}, source)
	})

	t.Run("search list without explicit path", func(t *testing.T) {
		source, err := findConfigSource("app", "")
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

		_, err := findConfigSource("app", "")
		require.ErrorIs(t, err, ErrConfigNotFound)
		require.ErrorIs(t, err, fs.ErrPermission)
	})
}
