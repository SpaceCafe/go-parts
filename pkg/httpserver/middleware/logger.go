package middleware

import (
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/log"
)

// LoggerOption configures the Logger middleware.
type LoggerOption func(*loggerOptions)

type loggerOptions struct {
	logQueryValues bool
}

// WithQueryValues makes Logger write the full query, values included. Query parameters often carry
// tokens or API keys, so only enable this when the logs are as protected as the credentials.
func WithQueryValues() LoggerOption {
	return func(opts *loggerOptions) {
		opts.logQueryValues = true
	}
}

// Logger provides an HTTP middleware that logs incoming requests using the specified logger. By
// default only the names of query parameters are logged, not their values; see WithQueryValues.
func Logger(logger log.Logger, opts ...LoggerOption) httpserver.Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	options := &loggerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			var query any = slices.Sorted(maps.Keys(req.URL.Query()))
			if options.logQueryValues {
				query = req.URL.Query()
			}

			logger.Info(
				"received request",
				"remote_addr", req.RemoteAddr,
				"method", req.Method,
				"scheme", req.URL.Scheme,
				"host", req.Host,
				"path", req.URL.Path,
				"query", query,
				"proto", req.Proto,
				"content_length", req.ContentLength,
				"user_agent", req.UserAgent(),
				"referer", req.Referer(),
			)
			next.ServeHTTP(resp, req)
		})
	}
}
