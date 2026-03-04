package httpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spacecafe/go-parts/pkg/typeconv"
)

var ErrInvalidFileHeader = errors.New("invalid file header")

func SaveBodyToFile(
	resp http.ResponseWriter,
	req *http.Request,
	magicBytes []byte,
) (dir, filePath string, clean func() error, err error) {
	magic := make([]byte, 0)

	if len(magicBytes) > 0 {
		magic = make([]byte, len(magicBytes))

		_, err = io.ReadFull(req.Body, magic)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)

			return "", "", nil, fmt.Errorf("failed to read file header: %w", err)
		}

		if !bytes.Equal(magic, magicBytes) {
			resp.WriteHeader(http.StatusBadRequest)

			return "", "", nil, ErrInvalidFileHeader
		}
	}

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		return "", "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	filePath = filepath.Join(tempDir, "input.pdf")

	file, err := os.Create(filePath) // #nosec G304
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	reader := io.MultiReader(bytes.NewReader(magic), req.Body)

	_, err = io.Copy(file, reader)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", nil, fmt.Errorf("failed to copy request to temp file: %w", err)
	}

	return tempDir, filePath, func() error { return os.RemoveAll(tempDir) }, nil
}

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

	return getFormValue[T](req.URL.Query().Get(key), defaultValue, validators...)
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

	return getFormValue[T](req.FormValue(key), defaultValue, validators...)
}

// getFormValue retrieves and converts a given value to the specified type T, with optional validation.
//
//nolint:ireturn // Generic function must return type parameter T.
func getFormValue[T any](formValue string, defaultValue T, validators ...func(T) error) (T, error) {
	if formValue == "" {
		return defaultValue, nil
	}

	value, err := typeconv.ConvertTo[T](formValue)
	if err != nil {
		return defaultValue, err
	}

	for _, validator := range validators {
		if validator == nil {
			continue
		}

		err = validator(value)
		if err != nil {
			return defaultValue, err
		}
	}

	return value, nil
}
