package validate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

var (
	ErrPathExist    = errors.New("validate: path must not exist")
	ErrPathNotExist = errors.New("validate: path must exist")
	ErrNotPerm      = errors.New("validate: path must have permissions")

	ErrNotDir  = errors.New("validate: path must be a directory")
	ErrNotFile = errors.New("validate: path must be a regular file")

	ErrNotReadable = errors.New("validate: path must be readable")
	ErrNotWritable = errors.New("validate: path must be writable")
	ErrPermExceeds = errors.New("validate: path must not have permissions beyond")
)

// Access bits accepted by accessible. They mirror `unix.R_OK`, `unix.W_OK` and `unix.X_OK` but are
// redeclared here, so the platform-independent validators stay free of a Unix-only import.
const (
	accessRead = 1 << iota
	accessWrite
	accessExec
)

// Traditional octal positions of the setuid, setgid and sticky bits. fs.FileMode keeps them far
// outside the low nine bits, so permBits has to map them back for callers who think in `chmod`.
const (
	permSetuid = fs.FileMode(0o4000)
	permSetgid = fs.FileMode(0o2000)
	permSticky = fs.FileMode(0o1000)
)

// DirExist validates that the provided path is an existing directory.
func DirExist[T ~string](value T) error {
	_, err := statDir(value)

	return err
}

// DirPerm validates that the provided path is an existing directory whose permission bits are
// exactly perm. perm is written the way `chmod` takes it, including the setuid, setgid and sticky
// bits, for example, `0o750` or `0o1777`.
func DirPerm[T ~string](perm fs.FileMode) func(T) error {
	return permExact(statDir[T], perm)
}

// DirPermMax validates that the provided path is an existing directory whose permission bits stay
// within perm. perm acts as a mask rather than an exact value, which is what "must not be group or
// world writable" style checks need: DirPermMax(0o750) accepts 0o700 and 0o750 but rejects 0o755.
func DirPermMax[T ~string](perm fs.FileMode) func(T) error {
	return permWithin(statDir[T], perm)
}

// DirRO validates that the provided path is an existing directory the current process can list and
// traverse. It asserts the presence of read access, not the absence of write access.
func DirRO[T ~string](value T) error {
	_, err := statDir(value)
	if err != nil {
		return err
	}

	// Execute permission is part of the check because a directory without it cannot be entered,
	// which leaves read permission alone useless.
	if !accessible(string(value), accessRead|accessExec) {
		return ErrNotReadable
	}

	return nil
}

// DirRW validates that the provided path is an existing directory the current process can list,
// traverse and create entries in.
func DirRW[T ~string](value T) error {
	err := DirRO(value)
	if err != nil {
		return err
	}

	if !accessible(string(value), accessWrite) {
		return ErrNotWritable
	}

	return nil
}

// FileExist validates that the provided path is an existing regular file.
func FileExist[T ~string](value T) error {
	_, err := statFile(value)

	return err
}

// FilePerm validates that the provided path is an existing regular file whose permission bits are
// exactly perm. perm is written the way `chmod` takes it, including the setuid, setgid and sticky
// bits, for example, `0o640` or `0o4755`.
func FilePerm[T ~string](perm fs.FileMode) func(T) error {
	return permExact(statFile[T], perm)
}

// FilePermMax validates that the provided path is an existing regular file whose permission bits
// stay within perm. perm acts as a mask rather than an exact value, which is what secret material
// needs: FilePermMax(0o600) accepts 0o400 and 0o600 but rejects the group readable 0o640.
func FilePermMax[T ~string](perm fs.FileMode) func(T) error {
	return permWithin(statFile[T], perm)
}

// FileRO validates that the provided path is an existing regular file the current process can read.
// It asserts the presence of read access, not the absence of write access.
func FileRO[T ~string](value T) error {
	_, err := statFile(value)
	if err != nil {
		return err
	}

	if !accessible(string(value), accessRead) {
		return ErrNotReadable
	}

	return nil
}

// FileRW validates that the provided path is an existing regular file the current process can read
// and write.
func FileRW[T ~string](value T) error {
	err := FileRO(value)
	if err != nil {
		return err
	}

	if !accessible(string(value), accessWrite) {
		return ErrNotWritable
	}

	return nil
}

// PathNotExist validates that nothing occupies the provided path.
func PathNotExist[T ~string](value T) error {
	// Lstat rather than Stat: a dangling symlink is reported as non-existent by Stat, yet mkdir
	// still fails with EEXIST on it, so the path is not free.
	_, err := os.Lstat(string(value))
	if err == nil {
		return ErrPathExist
	}

	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrPathExist, err.Error())
}

// permBits returns the mode of info in the octal layout `chmod` uses, so that a caller can express
// an expectation as 0o2750 or 0o1777. fs.FileMode.Perm alone would silently drop those three bits.
func permBits(info fs.FileInfo) fs.FileMode {
	mode := info.Mode()
	bits := mode.Perm()

	if mode&fs.ModeSetuid != 0 {
		bits |= permSetuid
	}

	if mode&fs.ModeSetgid != 0 {
		bits |= permSetgid
	}

	if mode&fs.ModeSticky != 0 {
		bits |= permSticky
	}

	return bits
}

// permExact builds a validator that requires the entry returned by resolve to carry exactly perm.
func permExact[T ~string](resolve func(T) (fs.FileInfo, error), perm fs.FileMode) func(T) error {
	return func(value T) error {
		info, err := resolve(value)
		if err != nil {
			return err
		}

		if bits := permBits(info); bits != perm {
			return fmt.Errorf("%w %#o, got %#o", ErrNotPerm, perm, bits)
		}

		return nil
	}
}

// permWithin builds a validator that requires the entry returned by resolve to set no bit outside
// perm.
func permWithin[T ~string](resolve func(T) (fs.FileInfo, error), perm fs.FileMode) func(T) error {
	return func(value T) error {
		info, err := resolve(value)
		if err != nil {
			return err
		}

		if bits := permBits(info); bits&^perm != 0 {
			return fmt.Errorf("%w %#o, got %#o", ErrPermExceeds, perm, bits)
		}

		return nil
	}
}

// stat returns the metadata of the entry at value. Symlinks are followed, so a link pointing at the
// expected kind of entry is accepted.
func stat[T ~string](value T) (fs.FileInfo, error) {
	info, err := os.Stat(string(value))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrPathNotExist
		}

		return nil, fmt.Errorf("%w: %s", ErrPathNotExist, err.Error())
	}

	return info, nil
}

// statDir asserts that value is an existing directory and returns its metadata.
func statDir[T ~string](value T) (fs.FileInfo, error) {
	info, err := stat(value)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, ErrNotDir
	}

	return info, nil
}

// statFile asserts that value is an existing regular file and returns its metadata.
func statFile[T ~string](value T) (fs.FileInfo, error) {
	info, err := stat(value)
	if err != nil {
		return nil, err
	}

	// IsRegular rather than !IsDir: a device node, socket or named pipe would pass the negative
	// check while behaving nothing like the file a caller of these validators intends to open.
	if !info.Mode().IsRegular() {
		return nil, ErrNotFile
	}

	return info, nil
}
