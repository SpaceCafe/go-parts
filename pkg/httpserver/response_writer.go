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
// client errors (4xx) are logged at info level and their detail is passed through. A non-Redacted
// error is unwrapped before logging, so the wrapping context does not duplicate the logged cause.
func (r *ResponseWriter) Abort(req *http.Request, code int, err error) {
	var redacted Redacted

	logErr := err
	if !errors.As(err, &redacted) {
		logErr = errors.Unwrap(err)
	}

	args := []any{"method", req.Method, "path", req.URL.Path, "status", code, "error", logErr}

	if code >= http.StatusInternalServerError {
		r.Log.Error("httpserver: request failed", args...)
		r.Error(r.ResponseWriter, req, code, nil)
	} else {
		r.Log.Info("httpserver: request failed", args...)
		r.Error(r.ResponseWriter, req, code, err)
	}
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
