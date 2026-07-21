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
	"strconv"

	"github.com/spacecafe/go-parts/pkg/httpserver/validate"
	"github.com/spacecafe/go-parts/pkg/typeconv"
)

var (
	ErrInvalidFileHeader = errors.New("httpserver: invalid file header")
	ErrReadFileHeader    = errors.New("httpserver: failed to read file header")
	ErrTempDirCreation   = errors.New("httpserver: failed to create temp dir")
	ErrTempFileCreation  = errors.New("httpserver: failed to create temp file")
	ErrTempFileCopy      = errors.New("httpserver: failed to copy request to temp file")
	ErrJSONBodyDecoding  = errors.New("httpserver: failed to decode JSON body")
)

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

			return "", "", nil, fmt.Errorf("%w: %w", ErrReadFileHeader, err)
		}

		if !bytes.Equal(magic, magicBytes) {
			resp.WriteHeader(http.StatusBadRequest)

			return "", "", nil, ErrInvalidFileHeader
		}
	}

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		return "", "", nil, fmt.Errorf("%w: %w", ErrTempDirCreation, err)
	}

	filePath = filepath.Join(tempDir, "input.pdf")

	file, err := os.Create(filePath) // #nosec G304
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", nil, fmt.Errorf("%w: %w", ErrTempFileCreation, err)
	}

	defer func() { _ = file.Close() }()

	// Recombine the already-read magic bytes with the rest of the body.
	reader := io.MultiReader(bytes.NewReader(magic), req.Body)

	_, err = io.Copy(file, reader)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)

		_ = os.RemoveAll(tempDir)

		return "", "", nil, fmt.Errorf("%w: %w", ErrTempFileCopy, err)
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
func GetJSONBody(req *http.Request, v any) error {
	if err := json.NewDecoder(req.Body).Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrJSONBodyDecoding, err)
	}

	type validator interface {
		Validate() error
	}

	if val, ok := v.(validator); ok {
		if err := val.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// RespondWithError sends an HTTP error response with the specified status code and error message.
func RespondWithError(resp http.ResponseWriter, code int, err error) {
	http.Error(resp, err.Error(), code)
}

// RespondWithProblem constructs and sends a problem+json compliant error response (RFC 7807)
// with the given status code and error details.
func RespondWithProblem(resp http.ResponseWriter, code int, err error) {
	h := resp.Header()
	h.Del("Content-Length")
	h.Set("Content-Type", "application/problem+json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	resp.WriteHeader(code)

	statusText := http.StatusText(code)
	errorText, _ := json.Marshal(err.Error())

	_, _ = resp.Write([]byte(`{"type": "/errors/` + typeconv.ToKebabCase(statusText) + `", "title": "` + statusText + `", "status": ` + strconv.Itoa(code) + `, "detail": "`))
	_, _ = resp.Write(errorText)
	_, _ = resp.Write([]byte(`"}`))
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
