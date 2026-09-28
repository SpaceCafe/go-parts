//go:build unix

package procrun

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	ExitCodeBase = 128

	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = ExitCodeBase + int(syscall.SIGKILL) // equals 137
)

// applyProcessAttributes applies the required process attributes to the given command. It cannot
// fail, so Run has no error path between creating the work dir and starting the process that would
// leave the directory behind.
func applyProcessAttributes(runner *Runner, cmd *exec.Cmd) {
	cmd.SysProcAttr = &unix.SysProcAttr{
		// Create a new process group for isolation.
		Setpgid: true,
	}

	// On timeout or cancellation, kill the whole process group instead of only the direct child, so
	// grandchildren do not survive. With Setpgid the group ID equals the child's PID.
	cmd.Cancel = func() error {
		return unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
	}

	runner.Log.Debug("procrun: applying process attributes")
}

// exitCode returns the exit code of a finished process. A process killed by a signal reports
// ExitCodeBase plus the signal number, like a shell does (137 for SIGKILL).
func exitCode(state *os.ProcessState) int {
	status, ok := state.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() {
		return ExitCodeBase + int(status.Signal())
	}

	return state.ExitCode()
}

// wasKilled reports whether the process was terminated by SIGKILL, which is what cmd.Cancel sends
// on timeout or cancellation.
func wasKilled(state *os.ProcessState) bool {
	status, ok := state.Sys().(syscall.WaitStatus)

	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}
