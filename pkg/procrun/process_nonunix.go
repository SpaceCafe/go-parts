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

// applyArguments configures a Runner's arguments based on its configuration.
func applyArguments(_ *Runner) error {
	return nil
}

// applyProcessAttributes applies the required process attributes to the given command.
func applyProcessAttributes(_ *Runner, _ *exec.Cmd) error {
	return nil
}

// checkCapabilities checks and logs if the required binaries are available.
func checkCapabilities(r *Runner) {
	r.Log.Warn("procrun: landlock LSM and rlimits are only available on Linux." +
		"Process's filesystem, network, and resource restrictions will not be applied!")
}

// getExitCode extracts and returns the appropriate exit code from the provided error.
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
