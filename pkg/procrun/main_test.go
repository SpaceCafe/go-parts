package procrun_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/procrun"
)

// TestMain builds cmd/landlock-restrict into a temporary directory and puts it first on PATH, so the
// tests that use the default Config do not depend on the binary being installed.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	binDir, err := os.MkdirTemp("", "procrun-test-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "procrun: cannot create temp bin dir:", err)

		return m.Run()
	}

	defer func() { _ = os.RemoveAll(binDir) }()

	//nolint:gosec,noctx // Fixed build of this module; TestMain has no test context.
	build := exec.Command(
		"go", "build", "-o", filepath.Join(binDir, procrun.DefaultLandlockBin),
		"github.com/spacecafe/go-parts/cmd/landlock-restrict",
	)
	build.Stderr = os.Stderr

	err = build.Run()
	if err != nil {
		// Tests that need the binary skip themselves through requireHelperBinaries.
		fmt.Fprintln(os.Stderr, "procrun: cannot build landlock-restrict:", err)
	} else {
		_ = os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	return m.Run()
}

// requireHelperBinaries skips the test when the default helper binaries cannot be found, for
// example on a platform where landlock-restrict does not build or prlimit is not installed.
func requireHelperBinaries(t *testing.T) {
	t.Helper()

	for _, bin := range []string{procrun.DefaultLandlockBin, procrun.DefaultPrlimitBin} {
		_, err := exec.LookPath(bin)
		if err != nil {
			t.Skipf("%s not available: %v", bin, err)
		}
	}
}
