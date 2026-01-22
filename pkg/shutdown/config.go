package shutdown

import (
	"errors"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
)

var (
	_ config.Defaultable = (*Config)(nil)
	_ config.Validatable = (*Config)(nil)

	ErrInvalidTimeout = errors.New("shutdown timeout must be positive")
)

type Config struct {
	// Timeout specifies the duration before the application is forcefully killed.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Force indicates whether to forcibly terminate the application without waiting for a graceful shutdown.
	Force bool `json:"force" yaml:"force"`
}

func (c *Config) SetDefaults() {
	c.Timeout = time.Second * 3 //nolint:mnd // Default timeout value
	c.Force = true
}

func (c *Config) Validate() error {
	if c.Timeout <= 0 {
		return ErrInvalidTimeout
	}

	return nil
}
