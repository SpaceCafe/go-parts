package httpserver

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
)

const (
	DefaultHost              = "127.0.0.1"
	DefaultReadTimeout       = time.Second * 30
	DefaultReadHeaderTimeout = time.Second * 10
	DefaultWriteTimeout      = time.Second * 30
	DefaultIdleTimeout       = time.Second * 120
	DefaultPort              = 8080
)

var (
	_ config.Defaultable = (*Config)(nil)
	_ config.Validatable = (*Config)(nil)

	ErrInvalidHost     = errors.New("httpserver host must be a valid network address")
	ErrInvalidBasePath = errors.New(
		"httpserver base path must be an absolute path without trailing slash",
	)
	ErrMissingCertFile = errors.New(
		"httpserver cert file must be specified if key file is specified",
	)
	ErrMissingKeyFile = errors.New(
		"httpserver key file must be specified if cert file is specified",
	)
	ErrUnreadableCertFile       = errors.New("httpserver cert file must be readable")
	ErrUnreadableKeyFile        = errors.New("httpserver key file must be readable")
	ErrInvalidReadTimeout       = errors.New("httpserver read timeout must be positive")
	ErrInvalidReadHeaderTimeout = errors.New("httpserver read header timeout must be positive")
	ErrInvalidPort              = errors.New("httpserver port must be between 1 and 65535")
)

// Config defines the essential parameters for serving an http Server.
type Config struct {
	// Host represents network host address.
	Host string `json:"host" yaml:"host"`

	// BasePath represents the prefixed path in the URL.
	BasePath string `json:"basePath" yaml:"basePath"`

	// CertFile represents the path to the certificate file.
	CertFile string `json:"certFile" yaml:"certFile"`

	// KeyFile represents the path to the key file.
	KeyFile string `json:"keyFile" yaml:"keyFile"`

	// ReadTimeout represents the maximum duration before timing out read of the request.
	ReadTimeout time.Duration `json:"readTimeout" yaml:"readTimeout"`

	// ReadHeaderTimeout represents the amount of time allowed to read request headers.
	ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`

	// WriteTimeout represents the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `json:"writeTimeout" yaml:"writeTimeout"`

	// IdleTimeout represents the maximum amount of time to wait for the next request when keep-alive is enabled.
	IdleTimeout time.Duration `json:"idleTimeout" yaml:"idleTimeout"`

	// Port specifies the port to be used for connections.
	Port int `json:"port" yaml:"port"`

	// EnableH2C indicates whether HTTP/2 Cleartext (H2C) protocol support is enabled for the Server.
	// Use this only if you have configured a reverse proxy that terminates TLS.
	EnableH2C bool `json:"enableH2C" yaml:"enableH2C"`
}

// SetDefaults initializes the default values for the relevant fields in the struct.
func (r *Config) SetDefaults() {
	r.Host = DefaultHost
	r.ReadTimeout = DefaultReadTimeout
	r.ReadHeaderTimeout = DefaultReadHeaderTimeout
	r.WriteTimeout = DefaultWriteTimeout
	r.IdleTimeout = DefaultIdleTimeout
	r.Port = DefaultPort
	r.EnableH2C = false
}

// Validate ensures the all necessary configurations are filled and within valid confines.
func (r *Config) Validate() error {
	var err error

	if r.Host == "" {
		return ErrInvalidHost
	}

	if r.BasePath != "" && (!path.IsAbs(r.BasePath) || strings.HasSuffix(r.BasePath, "/")) {
		return ErrInvalidBasePath
	}

	if r.ReadTimeout <= 0 {
		return ErrInvalidReadTimeout
	}

	if r.ReadHeaderTimeout <= 0 {
		return ErrInvalidReadHeaderTimeout
	}

	if r.Port <= 0 || r.Port > 65535 {
		return ErrInvalidPort
	}

	if r.CertFile == "" && r.KeyFile == "" {
		return nil
	}

	if r.CertFile == "" {
		return ErrMissingCertFile
	}

	if r.KeyFile == "" {
		return ErrMissingKeyFile
	}

	r.CertFile, err = filepath.Abs(r.CertFile)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMissingCertFile, err)
	}

	r.KeyFile, err = filepath.Abs(r.KeyFile)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMissingKeyFile, err)
	}

	_, err = os.Stat(r.CertFile)
	if err != nil {
		return ErrUnreadableCertFile
	}

	_, err = os.Stat(r.KeyFile)
	if err != nil {
		return ErrUnreadableKeyFile
	}

	return nil
}
