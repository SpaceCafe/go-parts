package procrun_test

import (
	"bytes"
	"context"
	"os"
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
