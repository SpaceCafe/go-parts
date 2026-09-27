// Package validate provides composable validators for configuration values and request input.
//
// # Filesystem validators
//
// The filesystem validators (DirExist, DirRO, DirRW, DirPerm, DirPermMax, FileExist, FileRO,
// FileRW, FilePerm, FilePermMax and PathNotExist) are configuration sanity checks, not security
// boundaries. They inspect the path at the moment they run, and most of them follow symlinks, so
// the file can be replaced, moved or re-permissioned before it is actually opened (a
// time-of-check to time-of-use race). Code that must be safe against a hostile filesystem has to
// check the opened file itself, for example with os.Root or by calling Stat on the *os.File.
package validate
