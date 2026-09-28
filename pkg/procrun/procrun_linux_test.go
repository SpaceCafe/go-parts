//go:build linux

package procrun_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/procrun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
