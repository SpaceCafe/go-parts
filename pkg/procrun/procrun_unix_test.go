//go:build unix

package procrun_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/procrun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Run_ExitCodes(t *testing.T) {
	t.Parallel()

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = ""
	cfg.PrlimitBin = ""
	require.NoError(t, cfg.Validate())

	runner := procrun.New(cfg)

	t.Run("exit code of the process", func(t *testing.T) {
		t.Parallel()

		res, err := runner.Run(
			t.Context(),
			&procrun.Command{Path: "sh", Args: []string{"-c", "exit 3"}},
		)
		require.ErrorIs(t, err, procrun.ErrProcessTermination)
		assert.Equal(t, 3, res.ExitCode)
		assert.NotErrorIs(t, res.Error, context.DeadlineExceeded)
	})

	t.Run("deadline kills the process", func(t *testing.T) {
		t.Parallel()

		res, err := runner.Run(t.Context(), &procrun.Command{
			Path:    "sleep",
			Args:    []string{"30"},
			Timeout: 50 * time.Millisecond,
		})
		require.ErrorIs(t, err, procrun.ErrProcessTermination)
		require.ErrorIs(t, res.Error, context.DeadlineExceeded)
		assert.Equal(t, procrun.ExitCodeSigKill, res.ExitCode)
	})

	t.Run("parent cancellation kills the process", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(50*time.Millisecond, cancel)

		res, err := runner.Run(ctx, &procrun.Command{Path: "sleep", Args: []string{"30"}})
		require.ErrorIs(t, err, procrun.ErrProcessTermination)
		require.ErrorIs(t, res.Error, context.Canceled)
		assert.Equal(t, procrun.ExitCodeSigKill, res.ExitCode)
	})

	t.Run("process killed by another signal", func(t *testing.T) {
		t.Parallel()

		res, err := runner.Run(
			t.Context(),
			&procrun.Command{Path: "sh", Args: []string{"-c", "kill -TERM $$"}},
		)
		require.ErrorIs(t, err, procrun.ErrProcessTermination)
		assert.Equal(t, procrun.ExitCodeBase+15, res.ExitCode)
	})
}

func TestRun_TempWorkDirWritableInNarrowedSandbox(t *testing.T) {
	t.Parallel()
	requireHelperBinaries(t)

	cfg := &procrun.Config{}
	cfg.SetDefaults()
	cfg.Restrictions.Strict = true
	cfg.Restrictions.RWDirs = []string{}
	cfg.Restrictions.ROFiles = []string{"/etc/ld.so.cache"}

	for _, dir := range []string{"/usr", "/lib", "/lib64", "/bin"} {
		_, err := os.Stat(dir)
		if err == nil {
			cfg.Restrictions.RODirs = append(cfg.Restrictions.RODirs, dir)
		}
	}

	runner := procrun.New(cfg)

	// Strict mode fails on a kernel without Landlock, where there is nothing to test.
	probe, err := runner.Run(t.Context(), &procrun.Command{Path: "/bin/true", Dir: "/"})
	if err != nil {
		t.Skipf("landlock not available: %v", err)
	}

	require.NoError(t, runner.Cleanup(probe))

	result, err := runner.Run(t.Context(), &procrun.Command{
		Path: "/bin/sh",
		Args: []string{"-c", "echo ok > out"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = runner.Cleanup(result) })

	content, err := os.ReadFile(filepath.Join(result.WorkDir, "out"))
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(content))
}
