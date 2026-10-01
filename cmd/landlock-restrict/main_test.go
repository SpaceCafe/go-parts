//go:build unix

package main

import (
	"bytes"
	"flag"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewNetConfig checks the handled rights. Landlock V4 knows only TCP bind and connect, so
// "Net: all" at V4 means exactly those two, and UDP (V9) is never handled.
func TestNewNetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		wants []string
		never []string
		opts  options
	}{
		{
			name:  "bind only",
			opts:  options{restrictBind: true},
			wants: []string{"Landlock V4", "bind_tcp"},
			never: []string{"connect_tcp", "udp"},
		},
		{
			name:  "connect only",
			opts:  options{restrictConnect: true},
			wants: []string{"Landlock V4", "connect_tcp"},
			never: []string{"bind_tcp", "udp"},
		},
		{
			name:  "both",
			opts:  options{restrictBind: true, restrictConnect: true},
			wants: []string{"Landlock V4", "Net: all"},
			never: []string{"udp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, err := newNetConfig(&tt.opts)
			require.NoError(t, err)

			for _, want := range tt.wants {
				assert.Contains(t, config.String(), want)
			}

			for _, never := range tt.never {
				assert.NotContains(t, config.String(), never)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"":           {},
		"/usr":       {"/usr"},
		"/usr:/lib":  {"/usr", "/lib"},
		"/usr::/lib": {"/usr", "/lib"},
		"/usr:":      {"/usr"},
	}

	for value, want := range tests {
		assert.Equal(t, want, splitList(value), value)
	}
}

//nolint:paralleltest // Replaces the standard logger's output and the kernelABIVersion seam.
func TestCheckKernelSupport_WarnsAboutDegradedRights(t *testing.T) {
	var output bytes.Buffer

	log.SetOutput(&output)

	abi := kernelABIVersion

	t.Cleanup(func() {
		log.SetOutput(os.Stderr)

		kernelABIVersion = abi
	})

	// v6 enforces filesystem and TCP rules, but not the finer rights up to v9.
	kernelABIVersion = func() (int, error) { return 6, nil }

	for _, strict := range []bool{true, false} {
		output.Reset()

		require.NoError(t, checkKernelSupport(&options{restrictFS: true, strict: strict}))
		assert.Contains(t, output.String(), "filesystem rules need v9", "strict=%v", strict)

		output.Reset()

		quiet := &options{restrictFS: true, strict: strict, quiet: true}

		require.NoError(t, checkKernelSupport(quiet))
		assert.Empty(t, output.String(), "quiet, strict=%v", strict)
	}

	// Strict mode still refuses a kernel that cannot enforce a requested kind at all.
	kernelABIVersion = func() (int, error) { return 3, nil }

	require.ErrorIs(
		t,
		checkKernelSupport(&options{restrictBind: true, strict: true}),
		errUnsupportedABI,
	)
}

func TestPathList_SetRejectsMissingPaths(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(existing, "missing")

	var paths pathList

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(&paths, "ro.dir", "")

	require.ErrorIs(t, paths.Set(existing+listSeparator+missing), fs.ErrNotExist)
	assert.Empty(t, paths, "a value with a missing path adds nothing")

	// The flag package formats the Set error with %v, so only its text survives Parse.
	err := flags.Parse([]string{"-ro.dir=" + missing})
	require.ErrorContains(t, err, "-ro.dir")
	require.ErrorContains(t, err, missing)

	require.NoError(t, flags.Parse([]string{"-ro.dir=" + existing, "-ro.dir="}))
	assert.Equal(t, pathList{existing}, paths)
}
