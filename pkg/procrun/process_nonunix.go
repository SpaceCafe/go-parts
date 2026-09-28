//go:build !unix

package procrun

import (
	"os"
	"os/exec"
)

const (
	// ExitCodeSigKill is the exit code of a process killed through os.Process.Kill, which calls
	// TerminateProcess with exit code 1 on Windows. Other platforms without signals behave alike.
	ExitCodeSigKill = 1
)

// applyProcessAttributes applies the required process attributes to the given command. There are
// none outside Unix.
func applyProcessAttributes(_ *Runner, _ *exec.Cmd) {}

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
