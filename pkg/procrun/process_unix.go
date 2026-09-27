//go:build unix

package procrun

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	ExitCodeBase = 128

	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = ExitCodeBase + int(syscall.SIGKILL) // equals 137

	listSeparator = string(os.PathListSeparator)
)

// applyArguments configures a Runner's arguments based on its configuration. A helper binary that
// is not configured is skipped, so its restrictions are not applied but the command still runs, as
// checkCapabilities warns.
//
// prlimit runs first, outside the sandbox, so its own binary and libraries need no allowlist entry.
// The limits it sets survive the exec into landlock-restrict and the target. landlock-restrict runs
// last, so only the target itself runs inside the sandbox.
func applyArguments(runner *Runner) error {
	if runner.cfg.PrlimitBin != "" {
		runner.args = append(runner.args, prlimitArgs(runner.cfg)...)
	}

	if runner.cfg.LandlockBin != "" {
		runner.args = append(runner.args, landlockArgs(runner.cfg)...)
	}

	runner.Log.Debug("procrun: created args to restrict processes", "args", runner.args)

	return nil
}

// applyProcessAttributes applies the required process attributes to the given command.
func applyProcessAttributes(runner *Runner, cmd *exec.Cmd) error {
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

	return nil
}

// checkCapabilities checks and logs if the required binaries are available.
func checkCapabilities(runner *Runner) {
	if runner.cfg.LandlockBin == "" {
		runner.Log.Warn("procrun: landlock-restrict binary not found. " +
			"Process's filesystem and network restrictions will not be applied! Please check or ignore if intended.")
	}

	if runner.cfg.LandlockBin != "" && !runner.cfg.Restrictions.Strict {
		runner.Log.Warn("procrun: landlock-restrict runs in best-effort mode. " +
			"On a kernel without Landlock the process runs unrestricted! Set Restrictions.Strict to fail instead.")
	}

	if runner.cfg.PrlimitBin == "" {
		runner.Log.Warn("procrun: prlimit binary not found. " +
			"Process's resource limits will not be applied! Please check or ignore if intended.")
	}
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

// landlockArgs constructs a list of command-line arguments based on the filesystem restrictions defined in the Config.
func landlockArgs(cfg *Config) []string {
	//nolint:mnd // Max number of arguments
	args := make([]string, 0, 8)

	args = append(
		args,
		cfg.LandlockBin,
		"-ro.file="+strings.Join(cfg.Restrictions.ROFiles, listSeparator),
		"-rw.file="+strings.Join(cfg.Restrictions.RWFiles, listSeparator),
		"-ro.dir="+strings.Join(cfg.Restrictions.RODirs, listSeparator),
		"-rw.dir="+strings.Join(cfg.Restrictions.RWDirs, listSeparator),
	)

	if cfg.Restrictions.RestrictBindTCP {
		args = append(args, "-tcp.bind="+joinPorts(cfg.Restrictions.BindTCP))
	}

	if cfg.Restrictions.RestrictConnectTCP {
		args = append(args, "-tcp.connect="+joinPorts(cfg.Restrictions.ConnectTCP))
	}

	if cfg.Restrictions.Strict {
		args = append(args, "-strict")
	}

	return append(args, "--")
}

// prlimitArgs constructs a list of command-line arguments based on the process resource limits defined in the Config.
func prlimitArgs(cfg *Config) []string {
	//nolint:mnd // Max number of arguments
	args := make([]string, 0, 8)

	args = append(
		args,
		cfg.PrlimitBin,
		"--core="+strconv.FormatUint(cfg.Limits.CoreDumpSize.Uint64(), 10),
	)

	if cfg.Limits.CPU > 0 {
		args = append(args, "--cpu="+strconv.FormatFloat(cfg.Limits.CPU.Seconds(), 'f', 0, 64))
	}

	if cfg.Limits.Memory > 0 {
		args = append(args, "--as="+strconv.FormatUint(cfg.Limits.Memory.Uint64(), 10))
	}

	if cfg.Limits.FileSize > 0 {
		args = append(args, "--fsize="+strconv.FormatUint(cfg.Limits.FileSize.Uint64(), 10))
	}

	if cfg.Limits.MaxOpenFiles > 0 {
		args = append(args, "--nofile="+strconv.FormatUint(cfg.Limits.MaxOpenFiles, 10))
	}

	if cfg.Limits.MaxProcesses > 0 {
		args = append(args, "--nproc="+strconv.FormatUint(cfg.Limits.MaxProcesses, 10))
	}

	args = append(args, "--")

	return args
}

func joinPorts(nums []int) string {
	var builder strings.Builder

	for i, n := range nums {
		if i > 0 {
			builder.WriteString(listSeparator)
		}

		builder.WriteString(strconv.Itoa(n))
	}

	return builder.String()
}
