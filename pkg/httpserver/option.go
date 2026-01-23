package httpserver

import (
	"net/http"

	"github.com/spacecafe/go-parts/pkg/log"
)

// Option is a functional option for configuring HTTPServer.
type Option func(*HTTPServer)

func WithHandler(handler http.Handler) Option {
	return func(s *HTTPServer) {
		s.Server.Handler = handler
	}
}

func WithLogger(logger log.Logger) Option {
	return func(s *HTTPServer) {
		s.Log = logger
	}
}
