package httpserver_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errCause = errors.New("disk full")

// recordingLogger keeps the "error" argument of the last log call.
type recordingLogger struct {
	mockLogger

	loggedErr any
}

func (l *recordingLogger) Error(_ string, args ...any) { l.record(args) }
func (l *recordingLogger) Info(_ string, args ...any)  { l.record(args) }

func (l *recordingLogger) record(args []any) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "error" {
			l.loggedErr = args[i+1]
		}
	}
}

func TestResponseWriter_Abort_LogsError(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("save upload: %w", errCause)

	tests := []struct {
		err        error
		wantLogged error
		name       string
		wantBody   string
		code       int
	}{
		{
			name:       "plain error",
			code:       http.StatusBadRequest,
			err:        errCause,
			wantLogged: errCause,
			wantBody:   "disk full\n",
		},
		{
			name:       "wrapped error keeps context",
			code:       http.StatusBadRequest,
			err:        wrapped,
			wantLogged: wrapped,
			wantBody:   "save upload: disk full\n",
		},
		{
			name:       "redacted error logs cause and hides it from client",
			code:       http.StatusBadRequest,
			err:        httpserver.Redact(errCause),
			wantLogged: errCause,
			wantBody:   "Bad Request\n",
		},
		{
			name:       "wrapped redacted error logs cause and hides it from client",
			code:       http.StatusBadRequest,
			err:        fmt.Errorf("load user: %w", httpserver.Redact(errCause)),
			wantLogged: errCause,
			wantBody:   "Bad Request\n",
		},
		{
			name:       "server error logs cause and hides it from client",
			code:       http.StatusInternalServerError,
			err:        errCause,
			wantLogged: errCause,
			wantBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &recordingLogger{}
			rec := httptest.NewRecorder()
			resp := &httpserver.ResponseWriter{
				ResponseWriter: rec,
				Log:            logger,
				Error:          httpserver.RenderErrorAsText,
			}

			resp.Abort(
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
				tt.code,
				tt.err,
			)

			assert.Equal(t, tt.wantLogged, logger.loggedErr)
			assert.Equal(t, tt.code, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestRedact(t *testing.T) {
	t.Parallel()

	require.NoError(t, httpserver.Redact(nil))

	err := httpserver.Redact(errCause)
	assert.Empty(t, err.Error())
	assert.ErrorIs(t, err, errCause)
}

func TestResponseWriter_Flush(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	var writer http.ResponseWriter = &httpserver.ResponseWriter{ResponseWriter: rec}

	flusher, ok := writer.(http.Flusher)
	require.True(t, ok)

	flusher.Flush()

	assert.True(t, rec.Flushed)
}

func TestResponseWriter_Hijack(t *testing.T) {
	t.Parallel()

	t.Run("supported", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(
			http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				hijacker, ok := http.ResponseWriter(&httpserver.ResponseWriter{ResponseWriter: resp}).(http.Hijacker)
				if !assert.True(t, ok) {
					return
				}

				conn, buf, err := hijacker.Hijack()
				if !assert.NoError(t, err) {
					return
				}

				_, _ = buf.WriteString("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")
				_ = buf.Flush()
				_ = conn.Close()
			}),
		)
		t.Cleanup(server.Close)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
		require.NoError(t, err)

		resp, err := server.Client().Do(req)
		require.NoError(t, err)

		_ = resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("not supported", func(t *testing.T) {
		t.Parallel()

		writer := &httpserver.ResponseWriter{ResponseWriter: httptest.NewRecorder()}

		_, _, err := writer.Hijack()

		require.ErrorIs(t, err, http.ErrNotSupported)
	})
}

func TestAbort_PlainResponseWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wantBody string
		code     int
	}{
		{
			name:     "client error passes detail through",
			code:     http.StatusBadRequest,
			wantBody: "disk full\n",
		},
		{
			name:     "server error hides detail from client",
			code:     http.StatusInternalServerError,
			wantBody: "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()

			httpserver.Abort(
				rec,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
				tt.code,
				errCause,
			)

			assert.Equal(t, tt.code, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}
