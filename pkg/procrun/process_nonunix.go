//go:build !unix

package procrun

import (
	"errors"
	"os"
	"os/exec"
)

const (
	// ExitCodeSigKill is the exit code of a process killed through os.Process.Kill, which calls
	// TerminateProcess with exit code 1 on Windows. Other platforms without signals behave alike.
	ExitCodeSigKill = 1
)

// ErrStrictUnsupported is returned by Run when Restrictions.Strict is set on a platform without
// Landlock.
var ErrStrictUnsupported = errors.New("procrun: strict mode is only supported on Linux")

// applyArguments configures a Runner's arguments based on its configuration. Without Landlock,
// strict mode cannot be honored, so it fails instead of running unrestricted.
func applyArguments(runner *Runner) error {
	if runner.cfg.Restrictions.Strict {
		return ErrStrictUnsupported
	}

	return nil
}

// applyProcessAttributes applies the required process attributes to the given command. There are
// none outside Unix.
func applyProcessAttributes(_ *Runner, _ *exec.Cmd) {}

// checkCapabilities checks and logs if the required binaries are available.
func checkCapabilities(r *Runner) {
	r.Log.Warn("procrun: landlock LSM and rlimits are only available on Linux. " +
		"Process's filesystem, network, and resource restrictions will not be applied!")
}

// exitCode returns the exit code of a finished process.
func exitCode(state *os.ProcessState) int {
	return state.ExitCode()
}

// wasKilled reports whether the process ended unsuccessfully. Without signals, a process killed by
// cmd.Cancel cannot be told apart from one that failed on its own, so the caller also checks the
// context.
func wasKilled(state *os.ProcessState) bool {
	return !state.Success()
}
