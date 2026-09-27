//go:build unix

package validate_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// mkFIFO creates a named pipe, the cheapest entry that exists and is neither a directory nor a
// regular file.
func mkFIFO(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, syscall.Mkfifo(path, 0o600))

	return path
}
