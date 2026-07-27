package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/spacecafe/go-parts/pkg/log"
	"github.com/spacecafe/go-parts/pkg/shutdown"
)

// StartupCheckTimeout is how long Start waits for the listener goroutine to report an early
// failure (for example, a port already in use) before treating the server as successfully started.
const StartupCheckTimeout = 100 * time.Millisecond

var (
	_ shutdown.Trackable = (*HTTPServer)(nil)

	ErrInvalidContext = errors.New("httpserver: context must not be nil or cancelled")
)

// HTTPServer wraps an http.Server together with its configuration, logger, and error renderer
// and manages its lifecycle through Start and Stop.
type HTTPServer struct {
	cfg *Config

	// Log receives lifecycle and request events, defaults to slog.Default.
	Log log.Logger

	// Server is the underlying http.Server.
	Server *http.Server

	// errorRenderer formats error responses, defaults to RenderErrorAsText unless set via an Option.
	errorRenderer ErrorRenderer
}

// New builds an HTTPServer from Config and applies the given options. TLS is enabled only when both
// Config.CertFile and Config.KeyFile are set, and H2C is enabled only when Config.EnableH2C is true.
// Options run last so they can override any derived default.
func New(cfg *Config, opts ...Option) *HTTPServer {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(cfg.EnableH2C)

	obj := &HTTPServer{
		cfg: cfg,
		Log: slog.Default(),
		Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			Protocols:         protocols,
		},
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		obj.Server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{{
				Certificate: [][]byte{[]byte(cfg.CertFile)},
				PrivateKey:  []byte(cfg.KeyFile),
			}},
			MinVersion: tls.VersionTLS12,
		}
	}

	for _, opt := range opts {
		opt(obj)
	}

	if obj.errorRenderer == nil {
		obj.errorRenderer = RenderErrorAsText
	}

	return obj
}

// Start launches the server in a background goroutine and returns once it is listening. It blocks
// only for StartupCheckTimeout, so an immediate bind failure surfaces as an error, while a later
// failure is logged rather than returned.
func (s *HTTPServer) Start(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrInvalidContext
	}

	errCh := make(chan error, 1)

	s.setupRouter()

	go func() {
		s.Log.Info(
			"httpserver: starting HTTP server",
			"host", s.cfg.Host,
			"port", s.cfg.Port,
			"protocols", s.Server.Protocols.String(),
		)

		if s.Server.TLSConfig == nil {
			errCh <- s.Server.ListenAndServe()
		} else {
			errCh <- s.Server.ListenAndServeTLS("", "")
		}
	}()

	// Wait briefly to catch early initialization errors.
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	case <-time.After(StartupCheckTimeout):
		go func() {
			err := <-errCh
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.Log.Error("httpserver: failed to run HTTP server", "error", err)
			} else {
				s.Log.Info("httpserver: stopped HTTP server")
			}
		}()

		return nil
	}
}

// Stop gracefully shuts down the server, waiting for in-flight requests to finish or until
// context.Context is cancelled.
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.Log.Info("httpserver: stopping HTTP server")

	return fmt.Errorf("httpserver: failed to stop HTTP server: %w", s.Server.Shutdown(ctx))
}

// setupRouter injects the server's logger and error renderer into the handler when it implements
// the corresponding interfaces, keeping the router aligned with the server's configuration.
func (s *HTTPServer) setupRouter() {
	if router, ok := s.Server.Handler.(Loggable); ok {
		router.SetLogger(s.Log)
	}

	if router, ok := s.Server.Handler.(ErrorRenderable); ok {
		router.SetErrorRenderer(s.errorRenderer)
	}
}
