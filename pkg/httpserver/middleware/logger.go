package middleware

import (
	"log/slog"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/log"
)

// Logger provides an HTTP middleware that logs incoming requests using the specified logger.
func Logger(logger log.Logger) httpserver.Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			logger.Info(
				"received request",
				"remote_addr", req.RemoteAddr,
				"method", req.Method,
				"scheme", req.URL.Scheme,
				"host", req.Host,
				"path", req.URL.Path,
				"query", req.URL.Query(),
				"proto", req.Proto,
				"content_length", req.ContentLength,
				"user_agent", req.UserAgent(),
				"referer", req.Referer(),
			)
			next.ServeHTTP(resp, req)
		})
	}
}
