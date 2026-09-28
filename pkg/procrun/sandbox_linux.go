//go:build linux

package procrun

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// sandboxSupported reports whether prlimit and landlock-restrict can restrict a process here.
const sandboxSupported = true

const listSeparator = string(os.PathListSeparator)

// sandboxArgs returns the helper binaries and their arguments that wrap a command. A helper binary
// that is not configured is skipped, so its restrictions are not applied but the command still
// runs, as checkCapabilities warns. extraRWDirs are granted read-write access on top of
// Restrictions.RWDirs, such as the temporary work dir of a run.
//
// prlimit runs first, outside the sandbox, so its own binary and libraries need no allowlist entry.
// The limits it sets survive the exec into landlock-restrict and the target. landlock-restrict runs
// last, so only the target itself runs inside the sandbox.
func sandboxArgs(cfg *Config, extraRWDirs ...string) []string {
	var args []string

	if cfg.PrlimitBin != "" {
		args = append(args, prlimitArgs(cfg)...)
	}

	if cfg.LandlockBin != "" {
		args = append(args, landlockArgs(cfg, extraRWDirs...)...)
	}

	return args
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

// landlockArgs returns the landlock-restrict invocation for cfg, granting extraRWDirs read-write
// access in addition to Restrictions.RWDirs.
func landlockArgs(cfg *Config, extraRWDirs ...string) []string {
	//nolint:mnd // Max number of arguments
	args := make([]string, 0, 8)

	args = append(
		args,
		cfg.LandlockBin,
		"-ro.file="+strings.Join(cfg.Restrictions.ROFiles, listSeparator),
		"-rw.file="+strings.Join(cfg.Restrictions.RWFiles, listSeparator),
		"-ro.dir="+strings.Join(cfg.Restrictions.RODirs, listSeparator),
		"-rw.dir="+strings.Join(
			append(slices.Clone(cfg.Restrictions.RWDirs), extraRWDirs...),
			listSeparator,
		),
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
		// RLIMIT_CPU counts whole seconds. Round down as documented, but never to 0, which would
		// kill the process immediately.
		seconds := max(1, int64(cfg.Limits.CPU/time.Second))
		args = append(args, "--cpu="+strconv.FormatInt(seconds, 10))
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

// joinPorts joins ports with listSeparator for a landlock-restrict flag.
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
