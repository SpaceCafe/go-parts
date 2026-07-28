//go:build unix

package validate

import (
	"golang.org/x/sys/unix"
)

// accessible reports whether the current process may access path with the given mode.
//
// faccessat(2) is asked instead of comparing the mode bits from os.Stat by hand, because only the
// kernel accounts for ACLs, read-only mounts and Landlock rules. AT_EACCESS switches the check to
// the effective ids, which are the ones the process will actually be judged by when it opens the
// directory; the default access(2) behaviour of consulting the real ids answers a different
// question as soon as the binary is setuid.
func accessible(path string, mode int) bool {
	var flags uint32

	if mode&accessRead != 0 {
		flags |= unix.R_OK
	}

	if mode&accessWrite != 0 {
		flags |= unix.W_OK
	}

	if mode&accessExec != 0 {
		flags |= unix.X_OK
	}

	return unix.Faccessat(unix.AT_FDCWD, path, flags, unix.AT_EACCESS) == nil
}
