//go:build !unix

package procrun

import (
	"errors"
	"os/exec"
)

const (
	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = 0xC0000005
)

func applyProcessAttributes(_ log.Logger, _ *exec.Cmd, _ *Command) error {
	return errors.ErrUnsupported
}

func applyProcessLimits(_ log.Logger, _ int, _ *Limits) error {
	return errors.ErrUnsupported
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return 1
}
