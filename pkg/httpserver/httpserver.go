package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/log"
	"github.com/spacecafe/go-parts/pkg/shutdown"
)

var (
	_ shutdown.Trackable = (*HTTPServer)(nil)

	ErrInvalidContext = errors.New("httpserver: context must not be nil or cancelled")
	ErrServerStopped  = errors.New("httpserver: server was stopped and cannot be started again")
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

	// stopped is set by Stop. An http.Server cannot be reused after Shutdown, so Start refuses to run.
	stopped atomic.Bool
}

// New builds an HTTPServer from Config and applies the given options. TLS is enabled only when both
// Config.CertFile and Config.KeyFile are set, and H2C is enabled only when Config.EnableH2C is true.
// Options run last so they can override any derived default. A nil cfg applies the defaults. An
// invalid cfg panics (see config.MustValidate).
func New(cfg *Config, opts ...Option) *HTTPServer {
	if cfg == nil {
		cfg = &Config{}
		cfg.SetDefaults()
	}

	config.MustValidate(cfg)

	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(cfg.EnableH2C)

	obj := &HTTPServer{
		cfg: cfg,
		Log: slog.Default(),
		Server: &http.Server{
			Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			Protocols:         protocols,
		},
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		// Certificates stay empty on purpose: Start passes the file paths to ServeTLS, which loads
		// and parses the key pair before accepting connections, so a bad file fails Start.
		obj.Server.TLSConfig = &tls.Config{
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

// Start binds the listen address and returns once the server accepts connections, which then runs
// in a background goroutine. Bind and key pair errors are returned, while a failure after startup
// is logged. A server cannot be started again after Stop; Start then fails with ErrServerStopped,
// and a new HTTPServer is needed.
func (s *HTTPServer) Start(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrInvalidContext
	}

	if s.stopped.Load() {
		return ErrServerStopped
	}

	s.setupRouter()

	s.Log.Info(
		"httpserver: starting HTTP server",
		"host", s.cfg.Host,
		"port", s.cfg.Port,
		"protocols", s.Server.Protocols.String(),
	)

	addr := s.Server.Addr
	if addr == "" {
		addr = ":http"
	}

	// Listening here instead of in ListenAndServe returns bind errors directly, without a timer.
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("httpserver: failed to listen on %s: %w", addr, err)
	}

	ready := &readyListener{Listener: listener, ready: make(chan struct{})}
	errCh := make(chan error, 1)

	go func() {
		if s.Server.TLSConfig == nil {
			errCh <- s.Server.Serve(ready)
		} else {
			errCh <- s.Server.ServeTLS(ready, s.cfg.CertFile, s.cfg.KeyFile)
		}
	}()

	// Serve and ServeTLS return before their first Accept when the key pair fails to load or the
	// server was already shut down, so whichever channel fires first decides the outcome.
	select {
	case err := <-errCh:
		// ServeTLS does not close the listener when the key pair fails to load.
		_ = listener.Close()

		// ErrServerClosed this early means the server was shut down before it ever served, for
		// example through Server.Shutdown, which Stop cannot see.
		if errors.Is(err, http.ErrServerClosed) {
			return ErrServerStopped
		}

		return err
	case <-ready.ready:
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
	s.stopped.Store(true)

	err := s.Server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("httpserver: failed to stop HTTP server: %w", err)
	}

	return nil
}

// readyListener closes ready on the first Accept call, which marks the point where Serve has passed
// its own startup checks and is accepting connections.
type readyListener struct {
	net.Listener

	ready chan struct{}
	once  sync.Once
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })

	return l.Listener.Accept() //nolint:wrapcheck // Serve inspects the raw error.
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
