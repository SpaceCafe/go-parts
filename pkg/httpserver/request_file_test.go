package httpserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errBodyRead is returned by failingReader so a copy failure can be provoked without depending on
// the filesystem.
var errBodyRead = errors.New("body read failed")

func TestFile_Move(t *testing.T) {
	t.Parallel()

	tests := []struct {
		targetDir func(*testing.T) string
		wantErr   error
		name      string
		filename  string
	}{
		{
			name: "existing directory",
			targetDir: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			filename: "output.bin",
		},
		{
			name: "missing directory",
			targetDir: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing")
			},
			filename: "output.bin",
			wantErr:  httpserver.ErrTargetDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
			require.NoError(t, file.Err)

			t.Cleanup(func() { _ = file.Cleanup() })

			sourceDir := file.Dir
			targetDir := tt.targetDir(t)

			err := file.Move(targetDir, tt.filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				// A failed move must leave the temporary file untouched.
				assert.Equal(t, sourceDir, file.Dir)
				assert.FileExists(t, file.Path)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, targetDir, file.Dir)
			assert.Equal(t, filepath.Join(targetDir, tt.filename), file.Path)
			assertContent(t, file.Path, "payload")

			// Move takes over ownership, so the temporary directory is gone and Cleanup is a no-op.
			assert.NoDirExists(t, sourceDir)
			require.NoError(t, file.Cleanup())
			assert.FileExists(t, file.Path)
		})
	}
}

func TestFile_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		data        string
		wantContent string
	}{
		{name: "string value", data: "\t \n \r \"payload\"", wantContent: `payload`},
		{name: "empty value", data: `""`, wantContent: ``},
		{name: "object value", data: `{"key":"value"}`, wantErr: httpserver.ErrWriteFile},
		{name: "null value", data: `null`, wantErr: httpserver.ErrWriteFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var payload struct {
				File httpserver.File `json:"file"`
			}

			err := json.Unmarshal([]byte(`{"file":`+tt.data+`}`), &payload)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, payload.File)

			t.Cleanup(func() { _ = payload.File.Cleanup() })
			require.NoError(t, payload.File.Err)
			assertContent(t, payload.File.Path, tt.wantContent)
		})
	}
}

func TestBase64File_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		data        string
		wantContent string
	}{
		{name: "base64 value", data: `"cGF5bG9hZA=="`, wantContent: `payload`},
		{name: "invalid base64 value", data: `"cGF5bG9hZA="`, wantErr: httpserver.ErrWriteFile},
		{name: "object value", data: `{"key":"value"}`, wantErr: httpserver.ErrWriteFile},
		{name: "null value", data: `null`, wantErr: httpserver.ErrWriteFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var payload struct {
				File httpserver.Base64File `json:"file"`
			}

			err := json.Unmarshal([]byte(`{"file":`+tt.data+`}`), &payload)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, payload.File)

			t.Cleanup(func() { _ = payload.File.Cleanup() })
			require.NoError(t, payload.File.Err)
			assertContent(t, payload.File.Path, tt.wantContent)
		})
	}
}

func TestGetFileFromBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		body        io.Reader
		wantErr     error
		name        string
		wantContent string
		magicBytes  []byte
		wantCode    int
	}{
		{
			name:        "without magic bytes",
			body:        strings.NewReader("payload"),
			wantContent: "payload",
		},
		{
			name:        "empty body",
			body:        strings.NewReader(""),
			wantContent: "",
		},
		{
			name:        "matching magic bytes",
			body:        strings.NewReader("\x89PNGpayload"),
			magicBytes:  []byte("\x89PNG"),
			wantContent: "\x89PNGpayload",
		},
		{
			name:       "mismatching magic bytes",
			body:       strings.NewReader("GIF8payload"),
			magicBytes: []byte("\x89PNG"),
			wantErr:    httpserver.ErrInvalidFileHeader,
			wantCode:   http.StatusUnsupportedMediaType,
		},
		{
			name:       "body shorter than magic bytes",
			body:       strings.NewReader("\x89P"),
			magicBytes: []byte("\x89PNG"),
			wantErr:    httpserver.ErrReadFileHeader,
			wantCode:   http.StatusUnsupportedMediaType,
		},
		{
			name:     "unreadable body",
			body:     failingReader{},
			wantErr:  httpserver.ErrWriteFile,
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			file := httpserver.GetFileFromBody(newBodyRequest(t, tt.body), tt.magicBytes)
			t.Cleanup(func() { _ = file.Cleanup() })

			require.NotNil(t, file.Cleanup)

			if tt.wantErr != nil {
				require.ErrorIs(t, file.Err, tt.wantErr)
				assert.Equal(t, tt.wantCode, file.Code)
				assert.Empty(t, file.Dir)
				assert.Empty(t, file.Path)
				assert.NoError(t, file.Cleanup())

				return
			}

			require.NoError(t, file.Err)
			assert.Zero(t, file.Code)
			assert.DirExists(t, file.Dir)
			assert.Equal(t, filepath.Join(file.Dir, "input"), file.Path)
			assertContent(t, file.Path, tt.wantContent)

			require.NoError(t, file.Cleanup())
			assert.NoDirExists(t, file.Dir)
		})
	}
}

// failingReader stands in for a request body that dies mid-transfer.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errBodyRead }

func assertContent(t *testing.T, path, want string) {
	t.Helper()

	content, err := os.ReadFile(path) // #nosec G304
	require.NoError(t, err)
	assert.Equal(t, want, string(content))
}

func newBodyRequest(t *testing.T, body io.Reader) *http.Request {
	t.Helper()

	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
}
