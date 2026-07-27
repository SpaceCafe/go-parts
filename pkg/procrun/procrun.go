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

	err := applyArguments(obj)
	if err != nil {
		obj.Log.Error("failed to apply arguments", "error", err)
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
			return fmt.Errorf("%w: %s", ErrCleanup, err.Error())
		}

		return nil
	}

	// Remove all content in the directory and preserve the directory.
	entries, err := os.ReadDir(result.WorkDir)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCleanup, err.Error())
	}

	for _, entry := range entries {
		path := filepath.Join(result.WorkDir, entry.Name())

		err = os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrCleanup, err.Error())
		}
	}

	return nil
}

// Run executes the cmd with configured resource limits.
func (r *Runner) Run(ctx context.Context, cmd *Command) (*Result, error) {
	r.Log.Debug("procrun: executing cmd", "cmd", cmd)

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

// createExecCommand creates the *exec.Cmd with all I/O and env wired up.
func (r *Runner) createExecCommand(ctx context.Context, cmd *Command, workDir string) *exec.Cmd {
	args := append(slices.Clone(r.args), cmd.Path)
	args = append(args, cmd.Args...)

	//nolint:gosec // G204: cmd.Path and cmd.Args are intentionally dynamic, this package is a process runner by design.
	execCmd := exec.CommandContext(ctx, args[0], args[1:]...)
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

	r.Log.Debug("procrun: creating temporary directory for cmd execution")

	workDir, err := os.MkdirTemp("", cmd.TempDirPattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrWorkDirCreation, err.Error())
	}

	result.WorkDir = workDir
	result.IsTempDir = true

	return result, nil
}
