//go:build unix

package procrun

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/spacecafe/go-parts/pkg/log"
	"golang.org/x/sys/unix"
)

const (
	ExitCodeBase = 128

	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = ExitCodeBase + int(syscall.SIGKILL) // equals 137
)

type limitEntry struct {
	name     string
	resource int
	value    uint64
	always   bool
}

func applyProcessAttributes(logger log.Logger, execCmd *exec.Cmd, _ *Command) error {
	execCmd.SysProcAttr = &unix.SysProcAttr{
		// Create a new process group for isolation.
		Setpgid: true,
	}

	logger.Debug("procrun: applying process attributes")

	return nil
}

// applyProcessLimits sets resource limits for a process identified by pid using the provided Limits configuration.
func applyProcessLimits(logger log.Logger, pid int, limits *Limits) error {
	entries := []limitEntry{
		{"RLIMIT_CPU", unix.RLIMIT_CPU, uint64(limits.CPU.Seconds()), false},
		{"RLIMIT_AS", unix.RLIMIT_AS, limits.Memory.Uint64(), false},
		{"RLIMIT_FSIZE", unix.RLIMIT_FSIZE, limits.FileSize.Uint64(), false},
		{"RLIMIT_NOFILE", unix.RLIMIT_NOFILE, limits.MaxOpenFiles, false},
		{"RLIMIT_NPROC", unix.RLIMIT_NPROC, limits.MaxProcesses, false},
		{"RLIMIT_CORE", unix.RLIMIT_CORE, limits.CoreDumpSize.Uint64(), true},
	}

	for _, entry := range entries {
		if entry.value > 0 || entry.always {
			err := setLimit(pid, entry.resource, entry.value, entry.name)
			if err != nil {
				return err
			}
		}
	}

	logger.Debug("procrun: applying process limits", "pid", pid, "limits", limits)

	return nil
}

// setLimit sets a resource limit for a given process by PID and resource type with a specified value and name.
func setLimit(pid, resource int, value uint64, name string) error {
	rlimit := &unix.Rlimit{Cur: value, Max: value}

	err := unix.Prlimit(pid, resource, rlimit, nil)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrResourceLimit, name, err)
	}

	return nil
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)

		if ok && exitErr.ExitCode() == -1 && status.Signaled() {
			return ExitCodeBase + int(status.Signal())
		}

		return exitErr.ExitCode()
	}

	return 1
}
