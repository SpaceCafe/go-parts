package httpserver

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/validate"
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

	ErrInvalidBasePath = errors.New("validate: value must be an absolute path without trailing slash")
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

	// IdleTimeout represents the maximum amount of time to wait for the next request when keep-alive is enabled.
	IdleTimeout time.Duration `json:"idleTimeout" yaml:"idleTimeout"`

	// ReadTimeout represents the maximum duration before timing out read of the request.
	ReadTimeout time.Duration `json:"readTimeout" yaml:"readTimeout"`

	// ReadHeaderTimeout represents the amount of time allowed to read request headers.
	ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`

	// WriteTimeout represents the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `json:"writeTimeout" yaml:"writeTimeout"`

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
	var errCertFile, errKeyFile error

	if r.CertFile != "" {
		r.CertFile, errCertFile = filepath.Abs(r.CertFile)
	}

	if r.KeyFile != "" {
		r.KeyFile, errKeyFile = filepath.Abs(r.KeyFile)
	}

	return errors.Join(
		validate.Validate("host", r.Host, validate.NotEmpty),
		validate.Validate("base path", r.BasePath, validate.NotEmpty, func(value string) error {
			if !path.IsAbs(value) || strings.HasSuffix(value, "/") {
				return ErrInvalidBasePath
			}
			return nil
		}),
		validate.Validate("idle timeout", r.IdleTimeout, validate.Positive),
		validate.Validate("read timeout", r.ReadTimeout, validate.Positive),
		validate.Validate("read header timeout", r.ReadHeaderTimeout, validate.Positive),
		validate.Validate("write timeout", r.WriteTimeout, validate.Positive),
		validate.Validate("port", r.Port, validate.Between(0, 65_535)),
		validate.Validate("cert file", errCertFile, validate.NoError),
		validate.Validate("key file", errKeyFile, validate.NoError),
	)
}
