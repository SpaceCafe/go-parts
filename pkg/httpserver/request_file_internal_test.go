package httpserver

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simulateCrossDevice makes link fail like it does across filesystems, so moveFile has to copy.
func simulateCrossDevice(t *testing.T) {
	t.Helper()

	link = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "link", Old: oldpath, New: newpath, Err: syscall.EXDEV}
	}

	t.Cleanup(func() { link = os.Link })
}

//nolint:paralleltest // Replaces the package-level link seam.
func TestMoveFile_CrossDevice(t *testing.T) {
	simulateCrossDevice(t)

	src := filepath.Join(t.TempDir(), "input")
	dest := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))

	require.NoError(t, moveFile(src, dest))

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(content))
	assert.NoFileExists(t, src)
}

//nolint:paralleltest // Replaces the package-level link seam.
func TestMoveFile_CrossDeviceKeepsExistingTarget(t *testing.T) {
	simulateCrossDevice(t)

	src := filepath.Join(t.TempDir(), "input")
	dest := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))
	require.NoError(t, os.WriteFile(dest, []byte("existing"), 0o600))

	require.ErrorIs(t, moveFile(src, dest), fs.ErrExist)

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(content))
	assert.FileExists(t, src)
}
