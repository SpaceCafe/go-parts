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
