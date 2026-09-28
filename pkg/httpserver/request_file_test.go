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
		targetDir      func(t *testing.T, sourceDir string) string
		wantErr        error
		name           string
		filename       string
		wantSourceGone bool
	}{
		{
			name: "existing directory",
			targetDir: func(t *testing.T, _ string) string {
				t.Helper()

				return t.TempDir()
			},
			filename:       "output.bin",
			wantSourceGone: true,
		},
		{
			name: "missing directory",
			targetDir: func(t *testing.T, _ string) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing")
			},
			filename: "output.bin",
			wantErr:  httpserver.ErrTargetDir,
		},
		{
			name: "empty target directory renames in place",
			targetDir: func(*testing.T, string) string {
				return ""
			},
			filename: "renamed.bin",
		},
		{
			name: "target directory equal to source",
			targetDir: func(_ *testing.T, sourceDir string) string {
				return sourceDir
			},
			filename: "renamed.bin",
		},
		{
			name: "target subdirectory of source",
			targetDir: func(t *testing.T, sourceDir string) string {
				t.Helper()

				subDir := filepath.Join(sourceDir, "sub")
				require.NoError(t, os.Mkdir(subDir, 0o750))

				return subDir
			},
			filename: "output.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
			require.NoError(t, file.Err)

			t.Cleanup(func() { _ = file.Cleanup() })

			sourceDir := file.Dir
			targetDir := tt.targetDir(t, sourceDir)

			wantDir := targetDir
			if wantDir == "" {
				wantDir = sourceDir
			}

			err := file.Move(targetDir, tt.filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				// A failed move must leave the temporary file untouched.
				assert.Equal(t, sourceDir, file.Dir)
				assert.FileExists(t, file.Path)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, wantDir, file.Dir)
			assert.Equal(t, filepath.Join(wantDir, tt.filename), file.Path)
			assertContent(t, file.Path, "payload")

			if tt.wantSourceGone {
				assert.NoDirExists(t, sourceDir)
			} else {
				assert.DirExists(t, sourceDir)
			}
		})
	}
}

func TestGetFileFromBody_TooLarge(t *testing.T) {
	t.Parallel()

	req := newBodyRequest(t, strings.NewReader("payload"))
	req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 3)

	file := httpserver.GetFileFromBody(req, nil)

	require.ErrorIs(t, file.Err, httpserver.ErrRequestTooLarge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, file.Code)
	assert.Empty(t, file.Dir)
}

func TestFile_Move_CleanupKeepsTargetDir(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	existingFile := filepath.Join(targetDir, "earlier.bin")
	require.NoError(t, os.WriteFile(existingFile, []byte("earlier"), 0o600))

	file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
	require.NoError(t, file.Err)

	sourceDir := file.Dir

	require.NoError(t, file.Move(targetDir, "output.bin"))
	require.NoError(t, file.Cleanup())

	assert.NoDirExists(t, sourceDir)
	assertContent(t, existingFile, "earlier")
	assertContent(t, filepath.Join(targetDir, "output.bin"), "payload")
}

func TestFile_Move_RefusesExistingTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup func(t *testing.T, targetPath string)
		name  string
	}{
		{
			name: "regular file",
			setup: func(t *testing.T, targetPath string) {
				t.Helper()

				require.NoError(t, os.WriteFile(targetPath, []byte("earlier"), 0o600))
			},
		},
		{
			name: "symlink to a file outside the target directory",
			setup: func(t *testing.T, targetPath string) {
				t.Helper()

				outside := filepath.Join(t.TempDir(), "outside.bin")
				require.NoError(t, os.WriteFile(outside, []byte("earlier"), 0o600))
				require.NoError(t, os.Symlink(outside, targetPath))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()
			targetPath := filepath.Join(targetDir, "output.bin")
			tt.setup(t, targetPath)

			file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
			require.NoError(t, file.Err)

			t.Cleanup(func() { _ = file.Cleanup() })

			sourcePath := file.Path

			require.ErrorIs(t, file.Move(targetDir, "output.bin"), httpserver.ErrTargetExists)

			// Both the existing entry (read through the symlink, if any) and the upload are intact.
			assertContent(t, targetPath, "earlier")
			assert.Equal(t, sourcePath, file.Path)
			assertContent(t, file.Path, "payload")
		})
	}
}

func TestFile_Move_OntoItself(t *testing.T) {
	t.Parallel()

	file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
	require.NoError(t, file.Err)

	t.Cleanup(func() { _ = file.Cleanup() })

	sourcePath := file.Path

	require.NoError(t, file.Move("", filepath.Base(sourcePath)))
	assert.Equal(t, sourcePath, file.Path)
	assertContent(t, file.Path, "payload")
}

func TestFile_Move_CleanupRemovesTempDirAfterRename(t *testing.T) {
	t.Parallel()

	file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
	require.NoError(t, file.Err)

	sourceDir := file.Dir

	require.NoError(t, file.Move("", "renamed.bin"))
	require.NoError(t, file.Cleanup())

	assert.NoDirExists(t, sourceDir)
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
		{name: "escaped value", data: `"a\/b\u0021\n"`, wantContent: "a/b!\n"},
		{name: "object value", data: `{"key":"value"}`, wantErr: httpserver.ErrInvalidFileValue},
		{name: "null value", data: `null`, wantErr: httpserver.ErrInvalidFileValue},
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

func TestFile_UnmarshalJSON_DuplicateKeyRemovesPreviousTempDir(t *testing.T) {
	t.Parallel()

	var payload struct {
		File httpserver.File `json:"file"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{"file":"first"}`), &payload))
	firstDir := payload.File.Dir

	require.NoError(t, json.Unmarshal([]byte(`{"file":"second"}`), &payload))
	t.Cleanup(func() { _ = payload.File.Cleanup() })

	assert.NoDirExists(t, firstDir)
	assertContent(t, payload.File.Path, "second")
}

func TestBase64File_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		data        string
		wantContent string
		wantCode    int
	}{
		{name: "base64 value", data: `"cGF5bG9hZA=="`, wantContent: `payload`},
		{
			name:     "invalid base64 value",
			data:     `"cGF5bG9hZA="`,
			wantErr:  httpserver.ErrInvalidBase64,
			wantCode: http.StatusBadRequest,
		},
		{name: "base64 with escaped slash", data: `"Pz8\/"`, wantContent: "???"},
		{name: "object value", data: `{"key":"value"}`, wantErr: httpserver.ErrInvalidFileValue},
		{name: "null value", data: `null`, wantErr: httpserver.ErrInvalidFileValue},
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

				if tt.wantCode != 0 {
					assert.Equal(t, tt.wantCode, payload.File.Code)
				}

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

func TestFile_Move_InvalidFilename(t *testing.T) {
	t.Parallel()

	for _, filename := range []string{"", ".", "..", "../escape.bin", "sub/file.bin", "/etc/passwd"} {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()

			file := httpserver.GetFileFromBody(newBodyRequest(t, strings.NewReader("payload")), nil)
			require.NoError(t, file.Err)

			t.Cleanup(func() { _ = file.Cleanup() })

			path := file.Path

			require.ErrorIs(t, file.Move(t.TempDir(), filename), httpserver.ErrInvalidFilename)
			assert.Equal(t, path, file.Path)
			assert.FileExists(t, file.Path)
		})
	}
}
