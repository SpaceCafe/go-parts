package xutil

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrSameFile is returned by CopyFile when src and dest refer to the same file.
var ErrSameFile = errors.New("xutil: source and destination are the same file")

// CopyFile copies the file at src to dest, creating or truncating dest as needed,
// and syncs the destination to disk before returning. dest gets the permission bits of src, so a
// private file stays private; an existing dest is changed to them as well.
func CopyFile(src, dest string) error {
	return copyFile(src, dest, os.O_TRUNC)
}

// CopyFileExclusive copies the file at src to a new file at dest and syncs it to disk. It fails with
// an error matching fs.ErrExist when anything already occupies dest, a symlink included, so it never
// replaces a file or writes through a link. Use it when dest is derived from untrusted input. dest
// gets the permission bits of src; if the copy fails, the partial dest is removed.
func CopyFileExclusive(src, dest string) error {
	return copyFile(src, dest, os.O_EXCL)
}

// copyFile implements CopyFile and CopyFileExclusive. createFlag is os.O_TRUNC to replace an
// existing dest or os.O_EXCL to refuse one.
func copyFile(src, dest string, createFlag int) (err error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sourceFile.Close()) }()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Opening dest with O_TRUNC would empty src first if both are the same file, for example via a
	// symlink or hard link, and the copy would then write nothing back. O_EXCL refuses any existing
	// dest, so it needs no check.
	if createFlag&os.O_TRUNC != 0 {
		destInfo, statErr := os.Stat(dest)
		if statErr == nil && os.SameFile(sourceInfo, destInfo) {
			return fmt.Errorf("%w: %s", ErrSameFile, dest)
		}
	}

	perm := sourceInfo.Mode().Perm()

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|createFlag, perm)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, destFile.Close())

		// With O_EXCL this call created dest, so a failed copy must not leave a partial file behind.
		if err != nil && createFlag&os.O_EXCL != 0 {
			_ = os.Remove(dest)
		}
	}()

	// OpenFile applies perm only when it creates the file, and then masked by the umask; Chmod sets
	// the exact bits in both cases.
	err = destFile.Chmod(perm)
	if err != nil {
		return err
	}

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
