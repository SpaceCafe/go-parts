package procrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/spacecafe/go-parts/pkg/log"
)

var (
	ErrInvalidCommandPath = errors.New("procrun: command path cannot be empty")
	ErrWorkDirCreation    = errors.New("procrun: failed to create working directory")
	ErrProcessStart       = errors.New("procrun: failed to start process")
	ErrCleanup            = errors.New("procrun: failed to cleanup")
	ErrProcessTermination = errors.New("procrun: process terminated unexpectedly")
)

// WaitDelay bounds how long Run waits for the output pipes to close after the process was killed or
// exited. It stops a grandchild that keeps the pipes open from blocking Run indefinitely.
const WaitDelay = 5 * time.Second

// MinimalEnv lists the variables a command inherits from the current process when Command.Env is
// nil and Config.InheritEnv is false. They let ordinary tools find binaries, locale data, the time
// zone and a temp directory without exposing the host service's secrets.
//
//nolint:gochecknoglobals // Read-only list of variable names.
var MinimalEnv = []string{"PATH", "HOME", "LANG", "LC_ALL", "TZ", "TMPDIR"}

// Command describes the program to execute and its execution environment.
type Command struct {
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	Path           string
	Dir            string
	TempDirPattern string
	Args           []string
	Env            []string
	Timeout        time.Duration
}

// Result contains information about the completed process execution.
type Result struct {
	Error     error
	WorkDir   string
	ExitCode  int
	Duration  time.Duration
	IsTempDir bool
}

// Runner executes processes with resource limits.
type Runner struct {
	// Log is the logger instance.
	Log log.Logger

	// setupErr is the error from applying the sandbox arguments. Run refuses to start processes
	// while it is set, so a sandbox that cannot be set up never degrades silently.
	setupErr error

	// cfg holds configuration settings.
	cfg *Config

	args []string
}

func New(cfg *Config, opts ...Option) *Runner {
	obj := &Runner{
		Log: slog.Default(),
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(obj)
	}

	obj.setupErr = applyArguments(obj)
	if obj.setupErr != nil {
		obj.Log.Error("procrun: failed to apply arguments", "error", obj.setupErr)
	}

	checkCapabilities(obj)

	return obj
}

// Cleanup removes the working directory if procrun created it as a temporary directory. A directory
// the caller supplied through Command.Dir is never touched.
func (r *Runner) Cleanup(result *Result) error {
	if result == nil {
		return fmt.Errorf("%w: result cannot be nil", ErrCleanup)
	}

	if !result.IsTempDir {
		return nil
	}

	err := os.RemoveAll(result.WorkDir)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCleanup, err.Error())
	}

	return nil
}

// Run executes the cmd with configured resource limits.
func (r *Runner) Run(ctx context.Context, cmd *Command) (*Result, error) {
	// Log only what identifies the command: Env often carries secrets.
	r.Log.Debug("procrun: executing cmd", "path", cmd.Path, "args", cmd.Args, "dir", cmd.Dir)

	if cmd.Path == "" {
		return nil, ErrInvalidCommandPath
	}

	if r.setupErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrProcessStart, r.setupErr)
	}

	result, err := r.setupWorkDir(cmd)
	if err != nil {
		return nil, err
	}

	if r.cfg.AutoCleanup {
		defer func() {
			cleanupErr := r.Cleanup(result)
			if cleanupErr != nil {
				r.Log.Error("procrun: failed to cleanup", "error", cleanupErr)
			}
		}()
	}

	cmdCtx, cancel := r.applyTimeout(ctx, cmd)
	if cancel != nil {
		defer cancel()
	}

	execCmd := r.createExecCommand(cmdCtx, cmd, result.WorkDir)

	err = applyProcessAttributes(r, execCmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProcessStart, err.Error())
	}

	err = execCmd.Start()
	if err != nil {
		result.Error = err

		return result, fmt.Errorf("%w: %s", ErrProcessStart, err.Error())
	}

	return r.awaitResult(cmdCtx, execCmd, result)
}

// applyTimeout wraps the context with a deadline if cmd.Timeout > 0.
func (r *Runner) applyTimeout(
	ctx context.Context,
	cmd *Command,
) (context.Context, context.CancelFunc) {
	if cmd.Timeout <= 0 {
		return ctx, nil
	}

	r.Log.Debug("procrun: applying timeout to cmd execution", "timeout", cmd.Timeout)

	return context.WithTimeout(ctx, cmd.Timeout)
}

// awaitResult waits for the process to complete and populates the result.
func (r *Runner) awaitResult(
	cmdCtx context.Context,
	execCmd *exec.Cmd,
	result *Result,
) (*Result, error) {
	startTime := time.Now()
	err := execCmd.Wait()
	result.Duration = time.Since(startTime)

	r.Log.Debug("procrun: cmd execution completed", "duration", result.Duration, "error", err)

	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		result.Error = context.DeadlineExceeded
		result.ExitCode = ExitCodeSigKill

		return result, fmt.Errorf("%w: %s", ErrProcessTermination, result.Error.Error())
	}

	if err != nil {
		result.Error = err
		result.ExitCode = getExitCode(err)

		return result, fmt.Errorf("%w: %s", ErrProcessTermination, err.Error())
	}

	return result, nil
}

// commandEnv returns the environment for cmd. An explicit Command.Env is used as is (an empty slice
// means no variables). A nil Env inherits everything with Config.InheritEnv, otherwise only the
// MinimalEnv variables that are set.
func (r *Runner) commandEnv(cmd *Command) []string {
	if cmd.Env != nil {
		return cmd.Env
	}

	if r.cfg.InheritEnv {
		return nil
	}

	env := make([]string, 0, len(MinimalEnv))

	for _, key := range MinimalEnv {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}

	return env
}

// createExecCommand creates the *exec.Cmd with all I/O and env wired up.
func (r *Runner) createExecCommand(ctx context.Context, cmd *Command, workDir string) *exec.Cmd {
	args := append(slices.Clone(r.args), cmd.Path)
	args = append(args, cmd.Args...)

	//nolint:gosec // G204: cmd.Path and cmd.Args are intentionally dynamic, this package is a process runner by design.
	execCmd := exec.CommandContext(ctx, args[0], args[1:]...)
	execCmd.Dir = workDir
	execCmd.Env = r.commandEnv(cmd)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	// Without WaitDelay, Wait blocks until every process holding the stdout or stderr pipe exits,
	// which a surviving grandchild can delay forever. After the delay the pipes are closed.
	execCmd.WaitDelay = WaitDelay

	return execCmd
}

// setupWorkDir prepares the working directory, creating a temp dir if needed.
func (r *Runner) setupWorkDir(cmd *Command) (*Result, error) {
	result := &Result{}

	if cmd.Dir != "" {
		result.WorkDir = cmd.Dir

		return result, nil
	}

	r.Log.Debug("procrun: creating temporary directory for cmd execution")

	workDir, err := os.MkdirTemp("", cmd.TempDirPattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrWorkDirCreation, err.Error())
	}

	result.WorkDir = workDir
	result.IsTempDir = true

	return result, nil
}
