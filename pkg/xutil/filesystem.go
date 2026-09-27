package xutil

import (
	"errors"
	"io"
	"os"
)

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
