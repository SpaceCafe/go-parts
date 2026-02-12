package procrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spacecafe/go-parts/pkg/log"
)

const (
	// ExitCodeSigKill is the exit status code for SIGKILL, indicating the container received a SIGKILL
	// by the underlying operating system.
	ExitCodeSigKill = 128 + int(syscall.SIGKILL) // equals 137
)

var (
	ErrInvalidCommandPath = errors.New("procrun: cmd path cannot be empty")
	ErrWorkDirCreation    = errors.New("procrun: failed to create working directory")
	ErrProcessStart       = errors.New("procrun: failed to start process")
	ErrResourceLimit      = errors.New("procrun: could not set resource limit")
	ErrCleanup            = errors.New("procrun: failed to cleanup")
	ErrProcessTermination = errors.New("procrun: process terminated unexpectedly")
)

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

	// cfg holds configuration settings.
	cfg *Config
}

func New(cfg *Config, opts ...Option) *Runner {
	obj := &Runner{
		Log: slog.Default(),
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// Cleanup removes the working directory if it was auto-created.
func (r *Runner) Cleanup(result *Result) error {
	if result == nil {
		return fmt.Errorf("%w: result cannot be nil", ErrCleanup)
	}

	if result.IsTempDir {
		err := os.RemoveAll(result.WorkDir)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCleanup, err)
		}

		return nil
	}

	// Remove all content in the directory and preserve the directory.
	entries, err := os.ReadDir(result.WorkDir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCleanup, err)
	}

	for _, entry := range entries {
		path := filepath.Join(result.WorkDir, entry.Name())

		err = os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCleanup, err)
		}
	}

	return nil
}

// Run executes the cmd with configured resource limits.
func (r *Runner) Run(ctx context.Context, cmd *Command) (*Result, error) {
	return r.RunWithLimits(ctx, cmd, &r.cfg.Limits)
}

// RunWithLimits executes the cmd with specific resource limits.
func (r *Runner) RunWithLimits(ctx context.Context, cmd *Command, limits *Limits) (*Result, error) {
	r.Log.Debug("executing cmd", "cmd", cmd, "limits", limits)

	if cmd.Path == "" {
		return nil, ErrInvalidCommandPath
	}

	result, err := r.setupWorkDir(cmd)
	if err != nil {
		return nil, err
	}

	if r.cfg.AutoCleanup {
		defer func() {
			cleanupErr := r.Cleanup(result)
			if cleanupErr != nil {
				r.Log.Error("failed to cleanup", "error", cleanupErr)
			}
		}()
	}

	cmdCtx, cancel := r.applyTimeout(ctx, cmd)
	if cancel != nil {
		defer cancel()
	}

	execCmd := r.buildExecCmd(cmdCtx, cmd, result.WorkDir)

	err = applyProcessAttributes(execCmd, cmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProcessStart, err)
	}

	err = execCmd.Start()
	if err != nil {
		result.Error = err

		return result, fmt.Errorf("%w: %w", ErrProcessStart, err)
	}

	err = applyProcessLimits(execCmd.Process.Pid, limits)
	if err != nil {
		_ = execCmd.Process.Kill()
		result.Error = fmt.Errorf("%w: %w", ErrResourceLimit, err)

		return result, result.Error
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

	r.Log.Debug("applying timeout to cmd execution", "timeout", cmd.Timeout)

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

	r.Log.Debug("cmd execution completed", "duration", result.Duration, "error", err)

	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		result.Error = context.DeadlineExceeded
		result.ExitCode = ExitCodeSigKill

		return result, fmt.Errorf("%w: %w", ErrProcessTermination, result.Error)
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err

			return result, fmt.Errorf("%w: %w", ErrProcessTermination, err)
		}
	}

	return result, nil
}

// buildExecCmd creates the *exec.Cmd with all I/O and env wired up.
func (r *Runner) buildExecCmd(ctx context.Context, cmd *Command, workDir string) *exec.Cmd {
	//nolint:gosec // G204: cmd.Path and cmd.Args are intentionally dynamic, this package is a process runner by design.
	execCmd := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	execCmd.Dir = workDir
	execCmd.Env = cmd.Env
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	return execCmd
}

// setupWorkDir prepares the working directory, creating a temp dir if needed.
func (r *Runner) setupWorkDir(cmd *Command) (*Result, error) {
	result := &Result{}

	if cmd.Dir != "" {
		result.WorkDir = cmd.Dir

		return result, nil
	}

	r.Log.Debug("creating temporary directory for cmd execution")

	workDir, err := os.MkdirTemp("", cmd.TempDirPattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkDirCreation, err)
	}

	result.WorkDir = workDir
	result.IsTempDir = true

	return result, nil
}
