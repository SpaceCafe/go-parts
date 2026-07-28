package validate_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/require"
)

func TestDirExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
	}{
		{name: "directory", setup: func(t *testing.T) string {
			t.Helper()

			return mkDir(t, 0o750)
		}},
		{
			name: "regular file",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o640)
			},
			wantErr: validate.ErrNotDir,
		},
		{
			name: "missing path",
			setup: func(t *testing.T) string {
				t.Helper()

				return missing(t)
			},
			wantErr: validate.ErrPathNotExist,
		},
		{
			name: "symlink to directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkSymlink(t, mkDir(t, 0o750))
			},
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkSymlink(t, missing(t))
			},
			wantErr: validate.ErrPathNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.DirExist(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestDirPerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
		perm    fs.FileMode
	}{
		{
			name: "exact match",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o750)
			},
			perm: 0o750,
		},
		{
			name: "broader than expected",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o755)
			},
			perm:    0o750,
			wantErr: validate.ErrNotPerm,
		},
		{
			name: "narrower than expected",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o700)
			},
			perm:    0o750,
			wantErr: validate.ErrNotPerm,
		},
		{
			name: "sticky bit in traditional position",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o777|fs.ModeSticky)
			},
			perm: 0o1777,
		},
		{
			name: "sticky bit not expected",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o777|fs.ModeSticky)
			},
			perm:    0o777,
			wantErr: validate.ErrNotPerm,
		},
		{
			name: "regular file",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o750)
			},
			perm:    0o750,
			wantErr: validate.ErrNotDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.DirPerm[string](tt.perm)(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestDirPermMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
		perm    fs.FileMode
	}{
		{
			name: "equal to mask",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o750)
			},
			perm: 0o750,
		},
		{
			name: "narrower than mask",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o700)
			},
			perm: 0o750,
		},
		{
			name: "world readable beyond mask",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o755)
			},
			perm:    0o750,
			wantErr: validate.ErrPermExceeds,
		},
		{
			name: "sticky bit beyond mask",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o700|fs.ModeSticky)
			},
			perm:    0o777,
			wantErr: validate.ErrPermExceeds,
		},
		{
			name: "missing path",
			setup: func(t *testing.T) string {
				t.Helper()

				return missing(t)
			},
			perm:    0o750,
			wantErr: validate.ErrPathNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.DirPermMax[string](tt.perm)(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestDirRO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func(*testing.T) string
		wantErr  error
		name     string
		unprivil bool
	}{
		{name: "readable and traversable", setup: func(t *testing.T) string {
			t.Helper()

			return mkDir(t, 0o500)
		}},
		{name: "also writable", setup: func(t *testing.T) string {
			t.Helper()

			return mkDir(t, 0o700)
		}},
		{
			name: "not traversable",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o600)
			},
			wantErr:  validate.ErrNotReadable,
			unprivil: true,
		},
		{
			name: "not readable",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o300)
			},
			wantErr:  validate.ErrNotReadable,
			unprivil: true,
		},
		{
			name: "regular file",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o640)
			},
			wantErr: validate.ErrNotDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.unprivil {
				requireUnprivileged(t)
			}

			err := validate.DirRO(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestDirRW(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func(*testing.T) string
		wantErr  error
		name     string
		unprivil bool
	}{
		{name: "full access", setup: func(t *testing.T) string {
			t.Helper()

			return mkDir(t, 0o700)
		}},
		{
			name: "read only",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o500)
			},
			wantErr:  validate.ErrNotWritable,
			unprivil: true,
		},
		{
			name: "write only",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o300)
			},
			wantErr:  validate.ErrNotReadable,
			unprivil: true,
		},
		{
			name: "missing path",
			setup: func(t *testing.T) string {
				t.Helper()

				return missing(t)
			},
			wantErr: validate.ErrPathNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.unprivil {
				requireUnprivileged(t)
			}

			err := validate.DirRW(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestFileExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
	}{
		{name: "regular file", setup: func(t *testing.T) string {
			t.Helper()

			return mkFile(t, 0o640)
		}},
		{
			name: "directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o750)
			},
			wantErr: validate.ErrNotFile,
		},
		{
			name: "named pipe",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFIFO(t)
			},
			wantErr: validate.ErrNotFile,
		},
		{
			name: "missing path",
			setup: func(t *testing.T) string {
				t.Helper()

				return missing(t)
			},
			wantErr: validate.ErrPathNotExist,
		},
		{
			name: "symlink to regular file",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkSymlink(t, mkFile(t, 0o640))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.FileExist(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestFilePerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
		perm    fs.FileMode
	}{
		{
			name: "exact match",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o640)
			},
			perm: 0o640,
		},
		{
			name: "broader than expected",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o644)
			},
			perm:    0o640,
			wantErr: validate.ErrNotPerm,
		},
		{
			name: "directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o640)
			},
			perm:    0o640,
			wantErr: validate.ErrNotFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.FilePerm[string](tt.perm)(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestFilePermMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
		perm    fs.FileMode
	}{
		{
			name: "read only secret",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o400)
			},
			perm: 0o600,
		},
		{
			name: "equal to mask",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o600)
			},
			perm: 0o600,
		},
		{
			name: "group readable secret",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o640)
			},
			perm:    0o600,
			wantErr: validate.ErrPermExceeds,
		},
		{
			name: "missing path",
			setup: func(t *testing.T) string {
				t.Helper()

				return missing(t)
			},
			perm:    0o600,
			wantErr: validate.ErrPathNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.FilePermMax[string](tt.perm)(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestFileRO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func(*testing.T) string
		wantErr  error
		name     string
		unprivil bool
	}{
		{name: "read only", setup: func(t *testing.T) string {
			t.Helper()

			return mkFile(t, 0o400)
		}},
		{name: "also writable", setup: func(t *testing.T) string {
			t.Helper()

			return mkFile(t, 0o600)
		}},
		{
			name: "write only",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o200)
			},
			wantErr:  validate.ErrNotReadable,
			unprivil: true,
		},
		{
			name: "directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o750)
			},
			wantErr: validate.ErrNotFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.unprivil {
				requireUnprivileged(t)
			}

			err := validate.FileRO(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

func TestFileRW(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func(*testing.T) string
		wantErr  error
		name     string
		unprivil bool
	}{
		{name: "read and write", setup: func(t *testing.T) string {
			t.Helper()

			return mkFile(t, 0o600)
		}},
		{
			name: "read only",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o400)
			},
			wantErr:  validate.ErrNotWritable,
			unprivil: true,
		},
		{
			name: "write only",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o200)
			},
			wantErr:  validate.ErrNotReadable,
			unprivil: true,
		},
		{
			name: "named pipe",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFIFO(t)
			},
			wantErr: validate.ErrNotFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.unprivil {
				requireUnprivileged(t)
			}

			err := validate.FileRW(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

// TestNamedStringType pins down that the validators are usable through Validate on a defined string
// type without an explicit instantiation, which is the way callers wire them into a config.
func TestNamedStringType(t *testing.T) {
	t.Parallel()

	type path string

	dir := path(mkDir(t, 0o750))

	require.NoError(
		t,
		validate.Validate(
			"dir",
			dir,
			validate.DirExist,
			validate.DirRW,
			validate.DirPerm[path](0o750),
		),
	)
	require.Error(t, validate.Validate("dir", dir, validate.PathNotExist))
}

func TestPathNotExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(*testing.T) string
		wantErr error
		name    string
	}{
		{name: "free path", setup: func(t *testing.T) string {
			t.Helper()

			return missing(t)
		}},
		{
			name: "occupied by directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkDir(t, 0o750)
			},
			wantErr: validate.ErrPathExist,
		},
		{
			name: "occupied by regular file",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkFile(t, 0o640)
			},
			wantErr: validate.ErrPathExist,
		},
		{
			name: "occupied by dangling symlink",
			setup: func(t *testing.T) string {
				t.Helper()

				return mkSymlink(t, missing(t))
			},
			wantErr: validate.ErrPathExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.PathNotExist(tt.setup(t))
			requireErr(t, tt.wantErr, err)
		})
	}
}

// missing returns a path inside a fresh temporary directory that nothing occupies.
func missing(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "missing")
}

// mkDir creates a directory carrying the exact perm.
func mkDir(t *testing.T, perm fs.FileMode) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dir")
	require.NoError(t, os.Mkdir(path, perm))
	require.NoError(t, os.Chmod(path, perm))

	t.Cleanup(func() {
		_ = os.Chmod(path, 0o700)
	})

	return path
}

// mkFIFO creates a named pipe, the cheapest entry that exists and is neither a directory nor a
// regular file.
func mkFIFO(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, syscall.Mkfifo(path, 0o600))

	return path
}

// mkFile creates a regular file carrying the exact perm.
func mkFile(t *testing.T, perm fs.FileMode) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(path, []byte("content"), perm))
	require.NoError(t, os.Chmod(path, perm))

	return path
}

// mkSymlink creates a symlink pointing at the target and returns the link's own path.
func mkSymlink(t *testing.T, target string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, path))

	return path
}

// requireErr asserts that err is wantErr, or that there is no error when wantErr is nil.
func requireErr(t *testing.T, wantErr, err error) {
	t.Helper()

	if wantErr == nil {
		require.NoError(t, err)

		return
	}

	require.ErrorIs(t, err, wantErr)
}

// requireUnprivileged skips cases that expect access to be denied. Root bypasses the permission
// bits the kernel would otherwise enforce, so those expectations only hold for a normal user.
func requireUnprivileged(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission denial is not observable as root")
	}
}
