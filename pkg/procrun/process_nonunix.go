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

func applyArguments(_ *Runner) error {
	return errors.ErrUnsupported
}
func applyProcessAttributes(_ *Runner, _ *exec.Cmd) error {
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
