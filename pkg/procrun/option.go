package procrun

import (
	"github.com/spacecafe/go-parts/pkg/log"
)

// Option is a functional option for configuring Runner.
type Option func(*Runner)

func WithLogger(logger log.Logger) Option {
	return func(s *Runner) {
		s.Log = logger
	}
}
