//go:build !unix

package validate

import (
	"io/fs"
	"os"
)

// Owner permission bits, the only ones the fallback below can meaningfully consult.
const (
	permOwnerRead  = fs.FileMode(0o400)
	permOwnerWrite = fs.FileMode(0o200)
	permOwnerExec  = fs.FileMode(0o100)
)

// accessible reports whether the current process may access path with the given mode.
//
// Without faccessat(2) there is no portable way to ask the kernel, so the owner permission bits are
// inspected instead. On Windows those bits only reflect the read-only attribute and say nothing
// about the ACLs that actually decide access, so a directory can be reported as writable while it
// is not. Callers that need certainty have to attempt the operation itself.
func accessible(path string, mode int) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	var want fs.FileMode

	if mode&accessRead != 0 {
		want |= permOwnerRead
	}

	if mode&accessWrite != 0 {
		want |= permOwnerWrite
	}

	if mode&accessExec != 0 {
		want |= permOwnerExec
	}

	return info.Mode().Perm()&want == want
}
