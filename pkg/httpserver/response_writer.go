package httpserver

import (
	"errors"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/log"
)

// ResponseWriter decorates an http.ResponseWriter with the logger and error renderer needed to
// terminate a request through Abort. The Router wraps every response in one before dispatching.
type ResponseWriter struct {
	http.ResponseWriter

	// Log records the request failures produced by Abort.
	Log log.Logger

	// Error renders the client-facing error response.
	Error ErrorRenderer
}

// Abort logs the failure and writes an error response for code. Server errors (5xx) are logged at
// error level, and their detail is withheld from the client to avoid leaking internals, whereas
// client errors (4xx) are logged at info level and their detail is passed through. A Redacted error
// renders as an empty string, so the error it wraps is logged in its place.
func (r *ResponseWriter) Abort(req *http.Request, code int, err error) {
	logErr := logError(err)
	args := []any{"method", req.Method, "path", req.URL.Path, "status", code, "error", logErr}

	if code >= http.StatusInternalServerError {
		r.Log.Error("httpserver: request failed", args...)
		r.Error(r.ResponseWriter, req, code, nil)
	} else {
		r.Log.Info("httpserver: request failed", args...)
		r.Error(r.ResponseWriter, req, code, err)
	}
}

// logError returns the error Abort should log: err itself, or for a Redacted error the error it
// wraps, because the redacted error's own message is empty.
func logError(err error) error {
	// errors.AsType cannot be used: Redacted does not embed error.
	var redacted Redacted
	if !errors.As(err, &redacted) {
		return err
	}

	if wrapper, ok := redacted.(interface{ Unwrap() error }); ok && wrapper.Unwrap() != nil {
		return wrapper.Unwrap()
	}

	return err
}

func (r *ResponseWriter) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func Abort(resp http.ResponseWriter, req *http.Request, code int, err error) {
	if resp, ok := resp.(*ResponseWriter); ok {
		resp.Abort(req, code, err)

		return
	}

	RenderErrorAsText(resp, req, code, err)
}
