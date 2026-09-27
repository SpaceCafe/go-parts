//go:build !unix

package validate_test

import "testing"

// mkFIFO skips the test: named pipes in the filesystem only exist on Unix.
func mkFIFO(t *testing.T) string {
	t.Helper()
	t.Skip("named pipes are not supported on this platform")

	return ""
}
