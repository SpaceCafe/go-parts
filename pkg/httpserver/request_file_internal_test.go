package httpserver

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Replaces the package-level rename seam.
func TestMoveFile_CrossDevice(t *testing.T) {
	rename = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
	}

	t.Cleanup(func() { rename = os.Rename })

	src := filepath.Join(t.TempDir(), "input")
	dest := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))

	require.NoError(t, moveFile(src, dest))

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(content))
	assert.NoFileExists(t, src)
}
