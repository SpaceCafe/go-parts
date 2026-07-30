package xutil

import (
	"errors"
	"io"
	"os"
)

// CopyFile copies the file at src to dest, creating or truncating dest as needed,
// and syncs the destination to disk before returning.
func CopyFile(src, dest string) (err error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sourceFile.Close()) }()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, destFile.Close()) }()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
