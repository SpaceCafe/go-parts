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

const StartupCheckTimeout = 100 * time.Millisecond

var (
	_ shutdown.Trackable = (*HTTPServer)(nil)

	ErrInvalidContext = errors.New("httpserver: context must not be nil or cancelled")
)

type HTTPServer struct {
	// Config contains configuration settings for the HTTP Server.
	cfg *Config

	Log log.Logger

	Server *http.Server
}

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

	return obj
}

func (s *HTTPServer) Start(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrInvalidContext
	}

	errCh := make(chan error, 1)

	go func() {
		s.Log.Info(
			"starting HTTP server",
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
				s.Log.Error("failed to run HTTP server", "error", err)
			} else {
				s.Log.Info("stopped HTTP server")
			}
		}()

		return nil
	}
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	s.Log.Info("stopping HTTP server")

	return fmt.Errorf("httpserver: failed to stop HTTP server: %w", s.Server.Shutdown(ctx))
}
