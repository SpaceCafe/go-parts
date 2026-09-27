package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/log"
)

// Recover returns middleware that turns a panic in a later handler into a 500 response. It logs the
// panic value with its stack trace itself and passes no error to httpserver.Abort, so the value
// never reaches the client, even without a Router.
// A panic with http.ErrAbortHandler is re-raised, so the server still aborts the connection
// silently as the handler intended. If the handler already started the response, the 500 status
// cannot replace it. A nil logger falls back to slog.Default.
func Recover(logger log.Logger) httpserver.Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			defer func() {
				value := recover()
				if value == nil {
					return
				}

				err, isErr := value.(error)
				if isErr && errors.Is(err, http.ErrAbortHandler) {
					panic(value)
				}

				logger.Error(
					"middleware: recovered from panic",
					"method", req.Method,
					"path", req.URL.Path,
					"panic", value,
					"stack", string(debug.Stack()),
				)
				httpserver.Abort(resp, req, http.StatusInternalServerError, nil)
			}()

			next.ServeHTTP(resp, req)
		})
	}
}
