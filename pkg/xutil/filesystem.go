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
func CopyFile(src, dest string) (err error) {
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
	// symlink or hard link, and the copy would then write nothing back.
	destInfo, err := os.Stat(dest)
	if err == nil && os.SameFile(sourceInfo, destInfo) {
		return fmt.Errorf("%w: %s", ErrSameFile, dest)
	}

	perm := sourceInfo.Mode().Perm()

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, destFile.Close()) }()

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
