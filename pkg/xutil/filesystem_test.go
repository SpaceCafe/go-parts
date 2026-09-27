package xutil_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacecafe/go-parts/pkg/xutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(t *testing.T) (src, dest string)
		name    string
		wantErr bool
	}{
		{
			name: "successful copy",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				return src, filepath.Join(dir, "dest")
			},
		},
		{
			name: "missing source",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()

				return filepath.Join(dir, "missing"), filepath.Join(dir, "dest")
			},
			wantErr: true,
		},
		{
			name: "destination directory does not exist",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				return src, filepath.Join(dir, "missing", "dest")
			},
			wantErr: true,
		},
		{
			name: "destination file already exists",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				dest := filepath.Join(dir, "dest")
				require.NoError(
					t,
					os.WriteFile(dest, []byte("stale content that is longer"), 0o600),
				)

				return src, dest
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src, dest := tt.setup(t)
			err := xutil.CopyFile(src, dest)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			want, err := os.ReadFile(src)
			require.NoError(t, err)

			got, err := os.ReadFile(dest)
			require.NoError(t, err)

			assert.Equal(t, want, got)
		})
	}
}

func TestCopyFile_Permissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not supported")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(src, []byte("s3cr3t"), 0o600))

	t.Run("new destination", func(t *testing.T) {
		t.Parallel()

		dest := filepath.Join(t.TempDir(), "copy")
		require.NoError(t, xutil.CopyFile(src, dest))

		info, err := os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("existing world-readable destination", func(t *testing.T) {
		t.Parallel()

		dest := filepath.Join(t.TempDir(), "copy")
		//nolint:gosec // The test needs a world-readable file to prove it is tightened.
		require.NoError(t, os.WriteFile(dest, []byte("old"), 0o644))
		require.NoError(t, xutil.CopyFile(src, dest))

		info, err := os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}

func TestCopyFile_SameFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "data")
	require.NoError(t, os.WriteFile(src, []byte("keep me"), 0o600))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Link(src, link))

	for _, dest := range []string{src, link, filepath.Join(dir, ".", "data")} {
		require.ErrorIs(t, xutil.CopyFile(src, dest), xutil.ErrSameFile, dest)
	}

	content, err := os.ReadFile(src)
	require.NoError(t, err)
	assert.Equal(t, "keep me", string(content))
}

func TestCopyFileExclusive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

	dest := filepath.Join(dir, "dest")
	require.NoError(t, xutil.CopyFileExclusive(src, dest))

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestCopyFileExclusive_RefusesExistingDest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))

	existing := filepath.Join(dir, "existing")
	require.NoError(t, os.WriteFile(existing, []byte("keep me"), 0o600))

	symlink := filepath.Join(dir, "symlink")
	require.NoError(t, os.Symlink(existing, symlink))

	dangling := filepath.Join(dir, "dangling")
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing"), dangling))

	for _, dest := range []string{existing, symlink, dangling, src} {
		require.ErrorIs(t, xutil.CopyFileExclusive(src, dest), fs.ErrExist, dest)
	}

	assertFileContent(t, existing, "keep me")
	assertFileContent(t, src, "new")
	assert.NoFileExists(t, filepath.Join(dir, "missing"))
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(content))
}
