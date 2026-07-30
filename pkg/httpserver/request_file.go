package httpserver

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidFileHeader = errors.New("httpserver: invalid file header")
	ErrReadFileHeader    = errors.New("httpserver: failed to read file header")
	ErrTargetDir         = errors.New("httpserver: failed to use target directory")
	ErrTempDirCreation   = errors.New("httpserver: failed to create temporary directory")
	ErrTempFileCreation  = errors.New("httpserver: failed to create temporary file")
	ErrWriteFile         = errors.New("httpserver: failed to write request to target file")
)

// File is the result of saving a request body to disk. It carries the status code and error to
// report alongside the location, so a handler can pass Code and Err straight to Abort. On failure
// Dir and Path are empty and everything already written has been removed.
type File struct {
	Err        error
	reader     io.Reader
	Cleanup    func() error
	Dir        string
	Path       string
	magicBytes []byte
	Code       int
}

func GetFileFromBody(req *http.Request, magicBytes []byte) *File {
	file := &File{Cleanup: noopCleanup, reader: req.Body}
	file.create(magicBytes)

	if file.Err != nil {
		return file
	}

	return file
}

// Move moves the file into the given directory under the filename and returns the resulting path.
// If the target directory is empty, the file is renamed.
func (f *File) Move(dir, filename string) (err error) {
	if dir == "" {
		dir = f.Dir
	} else {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrTargetDir, err.Error())
		}
	}

	targetPath := filepath.Join(dir, filename)

	err = os.Rename(f.Path, targetPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTargetDir, err.Error())
	}

	// Don't clean up if new dir is equal to or a subdirectory of old dir.
	if !strings.HasPrefix(dir+string(filepath.Separator), f.Dir+string(filepath.Separator)) &&
		f.Dir != dir {
		_ = f.Cleanup()
	}

	f.Dir = dir
	f.Path = targetPath

	return nil
}

func (f *File) UnmarshalJSON(data []byte) error {
	data, f.Err = extractJSONValue(data)
	if f.Err != nil {
		return f.Err
	}

	f.reader = bytes.NewReader(data)
	f.create(nil)

	return f.Err
}

func (f *File) create(magicBytes []byte) {
	err := f.verifyMagic(magicBytes)
	if err != nil {
		f.fail(http.StatusUnsupportedMediaType, err)

		return
	}

	f.Dir, err = os.MkdirTemp("", "*")
	if err != nil {
		f.fail(
			http.StatusInternalServerError,
			fmt.Errorf("%w: %s", ErrTempDirCreation, err.Error()),
		)

		return
	}

	f.Path = filepath.Join(f.Dir, "input")
	f.Cleanup = func() error { return os.RemoveAll(f.Dir) }

	err = f.write()
	if err != nil {
		f.fail(http.StatusInternalServerError, err)

		return
	}
}

// fail discards whatever has been written so far and turns the result into a failure carrying code
// and err.
func (f *File) fail(code int, err error) {
	_ = f.Cleanup()

	f.Cleanup = noopCleanup
	f.Err = err
	f.Dir = ""
	f.Path = ""
	f.Code = code
}

// verifyMagic reads len(magicBytes) bytes from the reader and checks that they match magicBytes.
// The bytes read are returned so they can be recombined with the rest of the body.
func (f *File) verifyMagic(magicBytes []byte) error {
	if len(magicBytes) == 0 {
		return nil
	}

	f.magicBytes = make([]byte, len(magicBytes))

	_, err := io.ReadFull(f.reader, f.magicBytes)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrReadFileHeader, err.Error())
	}

	if !bytes.Equal(f.magicBytes, magicBytes) {
		return ErrInvalidFileHeader
	}

	return nil
}

// write creates filePath and writes a prefix followed by the remaining body.
func (f *File) write() error {
	file, err := os.Create(f.Path) // #nosec G304
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTempFileCreation, err.Error())
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	_, err = io.Copy(file, io.MultiReader(bytes.NewReader(f.magicBytes), f.reader))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrWriteFile, err.Error())
	}

	return nil
}

type Base64File struct {
	File
}

func (f *Base64File) UnmarshalJSON(data []byte) error {
	data, f.Err = extractJSONValue(data)
	if f.Err != nil {
		return f.Err
	}

	f.reader = base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	f.create(nil)

	return f.Err
}

func extractJSONValue(data []byte) ([]byte, error) {
	// Check if the first non-whitespace character is a quote.
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		return data[1 : len(data)-1], nil
	}

	return nil, ErrWriteFile
}

// noopCleanup is used as File.Cleanup when there is nothing to remove.
func noopCleanup() error { return nil }
