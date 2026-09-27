package procrun

import (
	"github.com/spacecafe/go-parts/pkg/log"
)

// Option is a functional option for configuring Runner.
type Option func(*Runner)

// WithLogger sets a custom logger for the Runner to use for logging activities.
func WithLogger(logger log.Logger) Option {
	return func(runner *Runner) {
		runner.Log = logger
	}
}
