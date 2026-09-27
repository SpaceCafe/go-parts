package procrun_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/procrun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Run(t *testing.T) {
	t.Parallel()

	requireHelperBinaries(t)

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

	requireHelperBinaries(t)

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

	requireHelperBinaries(t)

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

	requireHelperBinaries(t)

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

//nolint:paralleltest // Uses t.Setenv.
func TestRunner_Run_Env(t *testing.T) {
	t.Setenv("PROCRUN_TEST_SECRET", "hunter2")
	t.Setenv("LANG", "C.UTF-8")

	tests := []struct {
		name       string
		wantExact  string
		env        []string
		inheritEnv bool
		wantSecret bool
		wantLang   bool
	}{
		{name: "nil env gets minimal set", wantLang: true},
		{name: "inherit env", inheritEnv: true, wantSecret: true, wantLang: true},
		{name: "explicit env", env: []string{"A=1"}, wantExact: "A=1\n"},
		{name: "empty env", env: []string{}, wantExact: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &procrun.Config{}
			cfg.SetDefaults()
			cfg.LandlockBin = ""
			cfg.PrlimitBin = ""
			cfg.InheritEnv = tt.inheritEnv
			require.NoError(t, cfg.Validate())

			var stdout bytes.Buffer

			_, err := procrun.New(cfg).Run(t.Context(), &procrun.Command{
				Path:   "/usr/bin/env",
				Env:    tt.env,
				Stdout: &stdout,
			})
			require.NoError(t, err)

			if tt.wantExact != "" || tt.env != nil {
				assert.Equal(t, tt.wantExact, stdout.String())

				return
			}

			assert.Equal(
				t,
				tt.wantSecret,
				strings.Contains(stdout.String(), "PROCRUN_TEST_SECRET="),
			)
			assert.Equal(t, tt.wantLang, strings.Contains(stdout.String(), "LANG=C.UTF-8"))
		})
	}
}

func TestConfig_Validate_StrictRequiresLandlock(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	cfg.Restrictions.Strict = true

	require.ErrorIs(t, cfg.Validate(), procrun.ErrStrictWithoutLandlock)
}

// recordingLogger keeps every Debug line as text.
type recordingLogger struct {
	lines []string
}

func (l *recordingLogger) Debug(msg string, args ...any) {
	l.lines = append(l.lines, fmt.Sprint(append([]any{msg}, args...)...))
}

func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}

func TestRunner_Run_DoesNotLogEnv(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	require.NoError(t, cfg.Validate())

	logger := &recordingLogger{}

	_, err := procrun.New(cfg, procrun.WithLogger(logger)).Run(t.Context(), &procrun.Command{
		Path: "true",
		Env:  []string{"DB_PASSWORD=hunter2"},
	})
	require.NoError(t, err)

	for _, line := range logger.lines {
		assert.NotContains(t, line, "hunter2")
	}
}

func TestConfig_Validate_PathListSeparator(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	cfg.Restrictions.RODirs = []string{"/data/a" + string(os.PathListSeparator) + "b"}

	require.ErrorIs(t, cfg.Validate(), procrun.ErrPathListSeparator)
}
