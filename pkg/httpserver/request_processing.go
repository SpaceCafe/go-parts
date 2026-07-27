package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/spacecafe/go-parts/pkg/validate"
)

var (
	ErrInvalidFileHeader = errors.New("httpserver: invalid file header")
	ErrReadFileHeader    = errors.New("httpserver: failed to read file header")
	ErrTargetDir         = errors.New("httpserver: failed to use target directory")
	ErrTempDirCreation   = errors.New("httpserver: failed to create temporary directory")
	ErrTempFileCreation  = errors.New("httpserver: failed to create temporary file")
	ErrTempFileCopy      = errors.New("httpserver: failed to copy request to temporary file")
	ErrJSONBodyDecoding  = errors.New("httpserver: failed to decode JSON body")
)

// SaveBodyToFile writes the request body to a file, optionally verifying a leading magic-byte
// header and moving the result into targetDir. When targetDir is empty, the file is kept in a
// temporary directory that the returned cleanup removes.
func SaveBodyToFile(
	req *http.Request,
	targetDir, filename string,
	magicBytes []byte,
) (dir, filePath string, clean func() error, code int, err error) {
	magic, err := verifyMagic(req.Body, magicBytes)
	if err != nil {
		return "", "", noopCleanup, http.StatusBadRequest, err
	}

	if filename == "" {
		filename = "input"
	}

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		return "", "", noopCleanup, http.StatusInternalServerError, fmt.Errorf(
			"%w: %s",
			ErrTempDirCreation,
			err.Error(),
		)
	}

	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	filePath = filepath.Join(tempDir, filename)

	err = writeBodyToFile(filePath, magic, req.Body)
	if err != nil {
		return "", "", noopCleanup, http.StatusInternalServerError, err
	}

	if targetDir != "" {
		var targetPath string

		targetPath, err = moveToTarget(filePath, targetDir, filename)
		if err != nil {
			return "", "", noopCleanup, http.StatusInternalServerError, err
		}

		return filepath.Dir(targetPath), targetPath, noopCleanup, http.StatusOK, nil
	}

	return tempDir, filePath, func() error { return os.RemoveAll(tempDir) }, http.StatusOK, nil
}

// verifyMagic reads len(magicBytes) bytes from the reader and checks that they match magicBytes.
// The bytes read are returned so they can be recombined with the rest of the body.
func verifyMagic(reader io.Reader, magicBytes []byte) ([]byte, error) {
	if len(magicBytes) == 0 {
		return nil, nil
	}

	magic := make([]byte, len(magicBytes))

	_, err := io.ReadFull(reader, magic)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrReadFileHeader, err.Error())
	}

	if !bytes.Equal(magic, magicBytes) {
		return nil, ErrInvalidFileHeader
	}

	return magic, nil
}

// writeBodyToFile creates filePath and writes a prefix followed by the remaining body.
func writeBodyToFile(filePath string, prefix []byte, body io.Reader) error {
	file, err := os.Create(filePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTempFileCreation, err.Error())
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	_, err = io.Copy(file, io.MultiReader(bytes.NewReader(prefix), body))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTempFileCopy, err.Error())
	}

	return nil
}

// moveToTarget moves filePath into targetDir under the filename and returns the resulting path.
func moveToTarget(filePath, targetDir, filename string) (string, error) {
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrTargetDir, err.Error())
	}

	targetPath := filepath.Join(targetDir, filename)

	err = os.Rename(filePath, targetPath)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrTargetDir, err.Error())
	}

	return targetPath, nil
}

// noopCleanup is used as SavedBodyMeta.Cleanup when there is nothing to remove.
func noopCleanup() error { return nil }

// GetQueryParam retrieves and converts a query parameter from an HTTP request to the specified
// type. If the key is empty or the form value is missing, the defaultValue is returned. The
// validators are optional and perform additional validation on the value.
//
//nolint:ireturn // Generic function must return type parameter T.
func GetQueryParam[T any](
	req *http.Request,
	key string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if key == "" {
		return defaultValue, nil
	}

	return getFormValue[T](key, req.URL.Query().Get(key), defaultValue, validators...)
}

// GetFormValue retrieves and converts a form value from an HTTP request to the specified type. If
// the key is empty or the form value is missing, the defaultValue is returned. The validators are
// optional and perform additional validation on the value.
//
//nolint:ireturn // Generic function must return type parameter T.
func GetFormValue[T any](
	req *http.Request,
	key string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if key == "" {
		return defaultValue, nil
	}

	return getFormValue[T](key, req.FormValue(key), defaultValue, validators...)
}

// GetJSONBody decodes the JSON-encoded body of an HTTP request into `v`. If `v` implements a
// Validate error method, it is called after successful decoding and any
// returned error is propagated to the caller.
func GetJSONBody(req *http.Request, target any) error {
	err := json.NewDecoder(req.Body).Decode(target)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrJSONBodyDecoding, err.Error())
	}

	type validator interface {
		Validate() error
	}

	if val, ok := target.(validator); ok {
		err = val.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// getFormValue retrieves and converts a given value to the specified type T, with optional
// validation. The key names the value in any error returned.
//
//nolint:ireturn // Generic function must return type parameter T.
func getFormValue[T any](
	key string,
	formValue string,
	defaultValue T,
	validators ...func(T) error,
) (T, error) {
	if formValue == "" {
		return defaultValue, nil
	}

	value, err := typeconv.ConvertTo[T](formValue)
	if err != nil {
		// Report the raw form value: the converted value is the zero value here, not the input
		// the caller needs to see.
		return defaultValue, &validate.ValidationError{Name: key, Value: formValue, Err: err}
	}

	err = validate.Validate(key, value, validators...)
	if err != nil {
		return defaultValue, err
	}

	return value, nil
}
