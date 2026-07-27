package httpserver

import (
	"net/http"

	"github.com/spacecafe/go-parts/pkg/log"
)

// Option is a functional option for configuring HTTPServer.
type Option func(*HTTPServer)

// WithErrorRenderer overrides the response renderer to display a custom error response format.
func WithErrorRenderer(renderer ErrorRenderer) Option {
	return func(s *HTTPServer) {
		s.errorRenderer = renderer
	}
}

// WithHandler sets a custom http.Handler for the HTTP server.
func WithHandler(handler http.Handler) Option {
	return func(s *HTTPServer) {
		s.Server.Handler = handler
	}
}

// WithLogger sets a custom logger for the HTTPServer to use for logging activities.
func WithLogger(logger log.Logger) Option {
	return func(s *HTTPServer) {
		s.Log = logger
	}
}
