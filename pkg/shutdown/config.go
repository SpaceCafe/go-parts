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

	// Force indicates whether to exit the process when the graceful shutdown times out, so services
	// that do not stop cannot keep it alive. A shutdown that completes in time never exits the
	// process; main must then return on its own, for example after Wait.
	Force bool `json:"force" yaml:"force"`
}

func (c *Config) SetDefaults() {
	c.Timeout = DefaultTimeout
	c.Force = true
}

func (c *Config) Validate() error {
	return validate.Validate("timeout", c.Timeout, validate.Positive)
}
