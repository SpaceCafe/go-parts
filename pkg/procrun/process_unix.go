//go:build unix

package procrun

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	ExitCodeBase = 128

	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = ExitCodeBase + int(syscall.SIGKILL) // equals 137
)

// applyArguments configures a Runner's arguments based on its configuration.
func applyArguments(r *Runner) error {
	r.args = landlockNetArgs(r.cfg)
	r.args = append(r.args, landlockArgs(r.cfg)...)
	r.args = append(r.args, prlimitArgs(r.cfg)...)

	r.Log.Debug("procrun: created args to restrict processes", "args", r.args)

	return nil
}

// applyProcessAttributes applies the required process attributes to the given command.
func applyProcessAttributes(runner *Runner, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &unix.SysProcAttr{
		// Create a new process group for isolation.
		Setpgid: true,
	}

	runner.Log.Debug("procrun: applying process attributes")

	return nil
}

// checkCapabilities checks and logs if the required binaries are available.
func checkCapabilities(runner *Runner) {
	if runner.cfg.LandlockBin == "" {
		runner.Log.Warn("procrun: landlock-restrict binary not found." +
			"Process's filesystem restrictions will not be applied! Please check or ignore if intended.")
	}

	if runner.cfg.LandlockNetBin == "" {
		runner.Log.Warn("procrun: landlock-restrict-net binary not found." +
			"Process's network restrictions will not be applied! Please check or ignore if intended.")
	}

	if runner.cfg.PrlimitBin == "" {
		runner.Log.Warn("procrun: prlimit binary not found." +
			"Process's resource limits will not be applied! Please check or ignore if intended.")
	}
}

// getExitCode extracts and returns the appropriate exit code from the provided error,
// with special handling for signals.
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

// landlockArgs constructs a list of command-line arguments based on the filesystem restrictions defined in the Config.
func landlockArgs(cfg *Config) []string {
	//nolint:mnd // Max number of arguments
	args := make(
		[]string,
		0,
		len(
			cfg.Restrictions.RODirs,
		)+len(
			cfg.Restrictions.RWDirs,
		)+len(
			cfg.Restrictions.ROFiles,
		)+len(
			cfg.Restrictions.RWFiles,
		)+6,
	)

	args = append(args, cfg.LandlockBin, "-ro")
	args = append(args, cfg.Restrictions.RODirs...)
	args = append(args, "-rw")
	args = append(args, cfg.Restrictions.RWDirs...)
	args = append(args, "-rofiles")
	args = append(args, cfg.Restrictions.ROFiles...)
	args = append(args, "-rwfiles")
	args = append(args, cfg.Restrictions.RWFiles...)
	args = append(args, "--")

	return args
}

// landlockNetArgs constructs a list of command-line arguments based on the network restrictions defined in the Config.
func landlockNetArgs(cfg *Config) []string {
	var args []string

	if cfg.Restrictions.RestrictBindTCP || cfg.Restrictions.RestrictConnectTCP {
		args = append(args, cfg.LandlockNetBin)

		if cfg.Restrictions.RestrictBindTCP {
			for _, port := range cfg.Restrictions.BindTCP {
				args = append(args, "-tcp.bind", strconv.Itoa(port))
			}
		}

		if cfg.Restrictions.RestrictConnectTCP {
			for _, port := range cfg.Restrictions.ConnectTCP {
				args = append(args, "-tcp.connect", strconv.Itoa(port))
			}
		}

		args = append(args, "--")
	}

	return args
}

// prlimitArgs constructs a list of command-line arguments based on the process resource limits defined in the Config.
func prlimitArgs(cfg *Config) []string {
	//nolint:mnd // Max number of arguments
	args := make([]string, 0, 8)

	args = append(args, cfg.PrlimitBin, fmt.Sprintf("--core=%d", cfg.Limits.CoreDumpSize.Uint64()))
	if cfg.Limits.CPU > 0 {
		args = append(args, fmt.Sprintf("--cpu=%.0f", cfg.Limits.CPU.Seconds()))
	}

	if cfg.Limits.Memory > 0 {
		args = append(args, fmt.Sprintf("--as=%d", cfg.Limits.Memory.Uint64()))
	}

	if cfg.Limits.FileSize > 0 {
		args = append(args, fmt.Sprintf("--fsize=%d", cfg.Limits.FileSize.Uint64()))
	}

	if cfg.Limits.MaxOpenFiles > 0 {
		args = append(args, fmt.Sprintf("--nofile=%d", cfg.Limits.MaxOpenFiles))
	}

	if cfg.Limits.MaxProcesses > 0 {
		args = append(args, fmt.Sprintf("--nproc=%d", cfg.Limits.MaxProcesses))
	}

	args = append(args, "--")

	return args
}
