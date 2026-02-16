//go:build !unix

package procrun

import (
	"errors"
	"os/exec"
)

func applyProcessAttributes(_ log.Logger, _ *exec.Cmd, _ *Command) error {
	return errors.ErrUnsupported
}

func applyProcessLimits(_ log.Logger, _ int, _ *Limits) error {
	return errors.ErrUnsupported
}
