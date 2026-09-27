package httpserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spacecafe/go-parts/pkg/xutil"
)

var (
	ErrInvalidFileHeader = errors.New("httpserver: invalid file header")
	ErrInvalidFileValue  = errors.New("httpserver: file value must be a JSON string")
	ErrInvalidFilename   = errors.New("httpserver: filename must be a single path element")
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

// GetFileFromBody saves the request body to a temporary file. The body is read without a limit of
// its own; wrap it with middleware.MaxBodySize to get File.Code 413 and ErrRequestTooLarge when a
// client sends too much.
func GetFileFromBody(req *http.Request, magicBytes []byte) *File {
	file := &File{Cleanup: noopCleanup, reader: req.Body}
	file.create(magicBytes)

	return file
}

// rename is os.Rename, replaceable in tests to simulate a cross-filesystem move.
//
//nolint:gochecknoglobals // Test seam for the EXDEV fallback.
var rename = os.Rename

// Move moves the file into the given directory under the filename and returns the resulting path.
// If the target directory is empty, the file is renamed. The filename must be a single path
// element, so a client-supplied name cannot escape dir. When dir is on another filesystem than the
// temporary directory, the file is copied and the original removed.
func (f *File) Move(dir, filename string) (err error) {
	if filename != filepath.Base(filename) || filename == "." || filename == ".." {
		return fmt.Errorf("%w: %q", ErrInvalidFilename, filename)
	}

	if dir == "" {
		dir = f.Dir
	} else {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrTargetDir, err)
		}
	}

	targetPath := filepath.Join(dir, filename)

	err = moveFile(f.Path, targetPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTargetDir, err)
	}

	// Don't clean up if new dir is equal to or a subdirectory of old dir. Once the file has left the
	// temporary directory, Cleanup must not touch it or the caller's target directory.
	if !strings.HasPrefix(dir+string(filepath.Separator), f.Dir+string(filepath.Separator)) &&
		f.Dir != dir {
		_ = f.Cleanup()
		f.Cleanup = noopCleanup
	}

	f.Dir = dir
	f.Path = targetPath

	return nil
}

// moveFile renames src to dest, falling back to copy and remove when they are on different
// filesystems (for example a tmpfs /tmp), where rename fails with EXDEV.
func moveFile(src, dest string) error {
	err := rename(src, dest)
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	err = xutil.CopyFile(src, dest)
	if err != nil {
		_ = os.Remove(dest)

		return err
	}

	return os.Remove(src)
}

func (f *File) UnmarshalJSON(data []byte) error {
	data, err := extractJSONValue(data)
	if err != nil {
		f.fail(http.StatusBadRequest, err)

		return err
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
			fmt.Errorf("%w: %w", ErrTempDirCreation, err),
		)

		return
	}

	f.Path = filepath.Join(f.Dir, "input")
	// Capture the directory now: Move rewrites f.Dir, and Cleanup must only ever remove the temp dir.
	tempDir := f.Dir
	f.Cleanup = func() error { return os.RemoveAll(tempDir) }

	err = f.write()
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, ErrRequestTooLarge) {
			code = http.StatusRequestEntityTooLarge
		}

		f.fail(code, err)

		return
	}
}

// fail discards whatever has been written so far and turns the result into a failure carrying code
// and err.
func (f *File) fail(code int, err error) {
	// Cleanup is nil on a zero File, which is what json.Unmarshal hands to UnmarshalJSON.
	if f.Cleanup != nil {
		_ = f.Cleanup()
	}

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
		return fmt.Errorf("%w: %w", ErrReadFileHeader, err)
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
		return fmt.Errorf("%w: %w", ErrTempFileCreation, err)
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	_, err = io.Copy(file, io.MultiReader(bytes.NewReader(f.magicBytes), f.reader))
	if err != nil {
		return wrapBodyError(ErrWriteFile, err)
	}

	return nil
}

type Base64File struct {
	File
}

func (f *Base64File) UnmarshalJSON(data []byte) error {
	data, err := extractJSONValue(data)
	if err != nil {
		f.fail(http.StatusBadRequest, err)

		return err
	}

	f.reader = base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	f.create(nil)

	return f.Err
}

// extractJSONValue decodes a JSON string, including escapes such as \/ or \u0000. Any other JSON
// value, null included, is rejected with ErrInvalidFileValue.
func extractJSONValue(data []byte) ([]byte, error) {
	var value *string

	err := json.Unmarshal(data, &value)
	if err != nil || value == nil {
		return nil, ErrInvalidFileValue
	}

	return []byte(*value), nil
}

// noopCleanup is used as File.Cleanup when there is nothing to remove.
func noopCleanup() error { return nil }
