package httpserver

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/log"
)

var (
	_ http.Flusher  = (*ResponseWriter)(nil)
	_ http.Hijacker = (*ResponseWriter)(nil)
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
// client errors (4xx) are logged at info level and their detail is passed through. When a Redacted
// error occurs anywhere in the chain, the client gets only the status text, and the error it wraps
// is logged in its place. A nil Log or Error
// falls back to slog.Default and RenderErrorAsText.
func (r *ResponseWriter) Abort(req *http.Request, code int, err error) {
	logger := r.Log
	if logger == nil {
		logger = slog.Default()
	}

	renderer := r.Error
	if renderer == nil {
		renderer = RenderErrorAsText
	}

	logErr, redacted := logError(err)
	args := []any{"method", req.Method, "path", req.URL.Path, "status", code, "error", logErr}

	if code >= http.StatusInternalServerError {
		logger.Error("httpserver: request failed", args...)
		renderer(r.ResponseWriter, req, code, nil)
	} else {
		logger.Info("httpserver: request failed", args...)

		// A Redacted error wrapped by fmt.Errorf still renders the outer context ("load user: "), so
		// the whole error is withheld and the renderer falls back to the status text.
		if redacted {
			err = nil
		}

		renderer(r.ResponseWriter, req, code, err)
	}
}

// logError returns the error Abort should log: err itself, or for a Redacted error the error it
// wraps, because the redacted error's own message is empty. It also reports whether a Redacted
// error occurs anywhere in the chain.
func logError(err error) (error, bool) {
	// errors.AsType cannot be used: Redacted does not embed error.
	var redacted Redacted
	if !errors.As(err, &redacted) {
		return err, false
	}

	if wrapper, ok := redacted.(interface{ Unwrap() error }); ok && wrapper.Unwrap() != nil {
		return wrapper.Unwrap(), true
	}

	return err, true
}

// Flush sends buffered data to the client, satisfying http.Flusher for code that type-asserts the
// writer (for example server-sent events). It does nothing when the underlying writer cannot flush.
func (r *ResponseWriter) Flush() {
	_ = http.NewResponseController(r.ResponseWriter).Flush()
}

// Hijack hands the connection over to the caller, satisfying http.Hijacker for code that
// type-asserts the writer (for example WebSocket upgraders). It returns an error wrapping
// http.ErrNotSupported when the underlying writer cannot be hijacked, such as on HTTP/2.
func (r *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	//nolint:wrapcheck // Pass the error through unchanged so errors.Is(err, http.ErrNotSupported) works.
	return http.NewResponseController(r.ResponseWriter).Hijack()
}

// Unwrap returns the underlying writer, so http.ResponseController reaches its optional methods.
func (r *ResponseWriter) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Abort terminates the request through ResponseWriter.Abort. When middleware has wrapped the
// Router's ResponseWriter, Abort follows the Unwrap chain to borrow its logger and error renderer,
// but still writes through resp, so the wrapping middleware sees the response. Without a
// ResponseWriter in the chain (no Router wraps the request), the logger and renderer fall back to
// their defaults, so the error is still logged and server error details (5xx) are still withheld
// from the client.
func Abort(resp http.ResponseWriter, req *http.Request, code int, err error) {
	if writer, ok := resp.(*ResponseWriter); ok {
		writer.Abort(req, code, err)

		return
	}

	writer := &ResponseWriter{ResponseWriter: resp}
	if inner := findResponseWriter(resp); inner != nil {
		writer.Log = inner.Log
		writer.Error = inner.Error
	}

	writer.Abort(req, code, err)
}

// findResponseWriter follows the Unwrap chain of resp, the convention http.ResponseController also
// relies on, and returns the first ResponseWriter it finds, or nil.
func findResponseWriter(resp http.ResponseWriter) *ResponseWriter {
	for {
		switch writer := resp.(type) {
		case *ResponseWriter:
			return writer
		case interface{ Unwrap() http.ResponseWriter }:
			resp = writer.Unwrap()
		default:
			return nil
		}
	}
}
