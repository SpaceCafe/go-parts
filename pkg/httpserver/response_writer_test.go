package httpserver_test

import (
	"errors"
	"fmt"
	"io"
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

// statusRecorder is a middleware-style wrapper that records the status written through it.
type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func TestAbort_WrappedResponseWriter(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	rec := httptest.NewRecorder()
	outer := &statusRecorder{ResponseWriter: &httpserver.ResponseWriter{
		ResponseWriter: rec,
		Log:            logger,
		Error:          httpserver.RenderErrorAsProblem,
	}}

	httpserver.Abort(
		outer,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
		http.StatusBadRequest,
		errCause,
	)

	assert.Equal(t, errCause, logger.loggedErr)
	assert.Equal(t, http.StatusBadRequest, outer.status)
	assert.Equal(t, "application/problem+json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestResponseWriter_Written(t *testing.T) {
	t.Parallel()

	tests := []struct {
		write func(writer *httpserver.ResponseWriter)
		name  string
		want  bool
	}{
		{name: "nothing written", write: func(*httpserver.ResponseWriter) {}},
		{
			name:  "informational status",
			write: func(writer *httpserver.ResponseWriter) { writer.WriteHeader(http.StatusEarlyHints) },
		},
		{
			name:  "final status",
			write: func(writer *httpserver.ResponseWriter) { writer.WriteHeader(http.StatusOK) },
			want:  true,
		},
		{
			name:  "body",
			write: func(writer *httpserver.ResponseWriter) { _, _ = writer.Write([]byte("a")) },
			want:  true,
		},
		{
			name:  "flush",
			write: func(writer *httpserver.ResponseWriter) { writer.Flush() },
			want:  true,
		},
		{
			name:  "failed hijack",
			write: func(writer *httpserver.ResponseWriter) { _, _, _ = writer.Hijack() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer := &httpserver.ResponseWriter{ResponseWriter: httptest.NewRecorder()}
			tt.write(writer)

			assert.Equal(t, tt.want, writer.Written())
		})
	}
}

func TestResponseWriter_Abort_AfterWrite(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	rec := httptest.NewRecorder()
	writer := &httpserver.ResponseWriter{ResponseWriter: rec, Log: logger}

	_, err := writer.Write([]byte("partial"))
	require.NoError(t, err)

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		writer.Abort(
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
			http.StatusInternalServerError,
			errCause,
		)
	})

	assert.Equal(t, errCause, logger.loggedErr)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "partial", rec.Body.String(), "Abort must not append to a started response")
}

func TestAbort_WrappedResponseWriter_AfterWrite(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	outer := &statusRecorder{ResponseWriter: &httpserver.ResponseWriter{
		ResponseWriter: rec,
		Log:            &recordingLogger{},
	}}

	_, err := outer.Write([]byte("partial"))
	require.NoError(t, err)

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		httpserver.Abort(
			outer,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
			http.StatusInternalServerError,
			errCause,
		)
	})

	assert.Zero(t, outer.status, "Abort must not send a second status")
	assert.Equal(t, "partial", rec.Body.String())
}

func TestRouter_AbortAfterWrite_AbortsConnection(t *testing.T) {
	t.Parallel()

	router := httpserver.NewRouter()
	router.HandleFunc("GET /", func(resp http.ResponseWriter, req *http.Request) {
		_, _ = resp.Write([]byte("partial"))

		// Send the partial body, so the client has started reading when the connection aborts.
		http.NewResponseController(resp).Flush()

		httpserver.Abort(resp, req, http.StatusInternalServerError, errCause)
	})

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)

	require.ErrorIs(t, err, io.ErrUnexpectedEOF, "the client must see a truncated response")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "partial", string(body))
}
