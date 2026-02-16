//go:build unix

package procrun

import (
	"fmt"
	"os/exec"

	"github.com/spacecafe/go-parts/pkg/log"
	"golang.org/x/sys/unix"
)

type limitEntry struct {
	name     string
	resource int
	value    uint64
	always   bool
}

func applyProcessAttributes(log log.Logger, execCmd *exec.Cmd, _ *Command) error {
	execCmd.SysProcAttr = &unix.SysProcAttr{
		// Create a new process group for isolation.
		Setpgid: true,
	}

	log.Debug("procrun: applying process attributes")

	return nil
}

// applyProcessLimits sets resource limits for a process identified by pid using the provided Limits configuration.
func applyProcessLimits(log log.Logger, pid int, limits *Limits) error {
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

	log.Debug("procrun: applying process limits", "pid", pid, "limits", limits)

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
