package procrun_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/procrun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd    *procrun.Command
		expect func(*testing.T, *procrun.Result, error)
		name   string
	}{
		{
			name: "valid cmd",
			cmd: &procrun.Command{
				Path: "echo",
				Args: []string{"hello"},
			},
			expect: func(t *testing.T, res *procrun.Result, err error) {
				t.Helper()

				require.NoError(t, err)
				require.NotNil(t, res)
				assert.Equal(t, 0, res.ExitCode)
			},
		},
		{
			name: "invalid cmd path",
			cmd: &procrun.Command{
				Path: "",
			},
			expect: func(t *testing.T, _ *procrun.Result, err error) {
				t.Helper()

				assert.ErrorIs(t, err, procrun.ErrInvalidCommandPath)
			},
		},
		{
			name: "timeout exceeded",
			cmd: &procrun.Command{
				Path:    "/bin/sleep",
				Args:    []string{"5"},
				Timeout: time.Second,
			},
			expect: func(t *testing.T, res *procrun.Result, err error) {
				t.Helper()

				require.Error(t, err)
				require.NotNil(t, res)
			},
		},
	}

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	require.NoError(t, cfg.Validate())
	runner := procrun.New(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := runner.Run(context.Background(), tt.cmd)
			tt.expect(t, res, err)
		})
	}
}

func TestRunner_Cleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectErr error
		result    *procrun.Result
		name      string
	}{
		{
			name: "valid temporary directory cleanup",
			result: &procrun.Result{
				WorkDir:   t.TempDir(),
				IsTempDir: true,
			},
			expectErr: nil,
		},
		{
			name: "non-temporary directory cleanup",
			result: &procrun.Result{
				WorkDir: t.TempDir(),
			},
			expectErr: nil,
		},
		{
			name:      "nil result",
			result:    nil,
			expectErr: procrun.ErrCleanup,
		},
	}

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	require.NoError(t, cfg.Validate())
	runner := procrun.New(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if tt.result != nil {
					_ = os.RemoveAll(tt.result.WorkDir)
				}
			}()

			err := runner.Cleanup(tt.result)
			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommand_StdinStdoutStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	cmd := &procrun.Command{
		Path:   "bash",
		Args:   []string{"-c", "echo stdin_data; echo >&2 stderr_data"},
		Stdin:  bytes.NewBufferString("stdin_data"),
		Stdout: &stdout,
		Stderr: &stderr,
	}

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	require.NoError(t, cfg.Validate())
	runner := procrun.New(cfg)

	result, err := runner.Run(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, stdout.String(), "stdin_data")
	assert.Contains(t, stderr.String(), "stderr_data")
}

func TestCommand_Timeout(t *testing.T) {
	t.Parallel()

	cmd := &procrun.Command{
		Path:    "sleep",
		Args:    []string{"10"},
		Timeout: time.Second,
	}

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	require.NoError(t, cfg.Validate())
	runner := procrun.New(cfg)

	result, err := runner.Run(context.Background(), cmd)
	require.Error(t, err)
	require.NotNil(t, result)
}

func TestRunner_Run_WithoutHelperBinaries(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	require.NoError(t, cfg.Validate())

	var stdout bytes.Buffer

	res, err := procrun.New(cfg).Run(t.Context(), &procrun.Command{
		Path:   "echo",
		Args:   []string{"hello"},
		Stdout: &stdout,
	})

	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "hello\n", stdout.String())
}

func TestRunner_AutoCleanup_KeepsCallerDir(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	cfg.AutoCleanup = true
	require.NoError(t, cfg.Validate())

	runner := procrun.New(cfg)

	callerDir := t.TempDir()
	keptFile := filepath.Join(callerDir, "keep.txt")
	require.NoError(t, os.WriteFile(keptFile, []byte("data"), 0o600))

	res, err := runner.Run(t.Context(), &procrun.Command{Path: "true", Dir: callerDir})
	require.NoError(t, err)
	assert.False(t, res.IsTempDir)
	assert.FileExists(t, keptFile)

	res, err = runner.Run(t.Context(), &procrun.Command{Path: "true"})
	require.NoError(t, err)
	assert.True(t, res.IsTempDir)
	assert.NoDirExists(t, res.WorkDir)
}

func TestRunner_Run_TimeoutKillsProcessGroup(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	require.NoError(t, cfg.Validate())

	var stdout bytes.Buffer

	begin := time.Now()

	// The background sleep inherits stdout. Killing only sh would leave it holding the pipe open.
	res, err := procrun.New(cfg).Run(t.Context(), &procrun.Command{
		Path:    "sh",
		Args:    []string{"-c", "sleep 30 & sleep 30"},
		Stdout:  &stdout,
		Timeout: 200 * time.Millisecond,
	})

	require.ErrorIs(t, err, procrun.ErrProcessTermination)
	require.NotNil(t, res)
	assert.Less(t, time.Since(begin), procrun.WaitDelay, "Run must not wait for the grandchild")
}
