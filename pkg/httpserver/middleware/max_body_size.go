package middleware

import (
	"io"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/httpserver"
)

// limitedBody is the request body installed by MaxBodySize. It keeps the original body so a later
// MaxBodySize in the chain replaces the limit instead of nesting under it.
type limitedBody struct {
	io.ReadCloser

	original io.ReadCloser
}

// MaxBodySize limits the request body to limit bytes. Reading past the limit fails, and
// httpserver.GetJSONBody and httpserver.GetFileFromBody then report httpserver.ErrRequestTooLarge
// (GetFileFromBody also sets File.Code to 413).
//
// A MaxBodySize further down the chain replaces the limit rather than nesting under it, so a route
// can raise a global limit (for example for uploads). A limit of zero or less removes the
// limit. The Content-Length header is not checked up front: doing so in a global middleware would reject
// requests before a route could raise the limit.
func MaxBodySize(limit int64) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			original := req.Body
			if body, ok := original.(*limitedBody); ok {
				original = body.original
			}

			if limit <= 0 {
				req.Body = original
			} else {
				req.Body = &limitedBody{
					ReadCloser: http.MaxBytesReader(resp, original, limit),
					original:   original,
				}
			}

			next.ServeHTTP(resp, req)
		})
	}
}
