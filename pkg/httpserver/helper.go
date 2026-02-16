package httpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

var ErrInvalidFileHeader = errors.New("invalid file header")

func SaveBodyToFile(
	resp http.ResponseWriter,
	req *http.Request,
	magicBytes []byte,
) (dir, filePath string, err error) {
	magic := make([]byte, 0)

	if len(magicBytes) > 0 {
		magic = make([]byte, len(magicBytes))

		_, err = io.ReadFull(req.Body, magic)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)

			return "", "", fmt.Errorf("failed to read file header: %w", err)
		}

		if !bytes.Equal(magic, magicBytes) {
			resp.WriteHeader(http.StatusBadRequest)

			return "", "", ErrInvalidFileHeader
		}
	}

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	filePath = filepath.Join(tempDir, "input.pdf")

	file, err := os.Create(filePath)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	reader := io.MultiReader(bytes.NewReader(magic), req.Body)

	_, err = io.Copy(file, reader)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", fmt.Errorf("failed to copy request to temp file: %w", err)
	}

	return tempDir, filePath, nil
}
