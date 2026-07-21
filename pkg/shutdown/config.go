package shutdown

import (
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/validate"
)

const DefaultTimeout = time.Second * 3

var (
	_ config.Defaultable = (*Config)(nil)
	_ config.Validatable = (*Config)(nil)
)

type Config struct {
	// Timeout specifies the duration before the application is forcefully killed.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Force indicates whether to forcibly terminate the application without waiting for a graceful shutdown.
	Force bool `json:"force" yaml:"force"`
}

func (c *Config) SetDefaults() {
	c.Timeout = DefaultTimeout
	c.Force = true
}

func (c *Config) Validate() error {
	return validate.Validate("timeout", c.Timeout, validate.Positive)
}
