// Package procrun runs external commands with resource limits and a Landlock sandbox.
//
// A Runner executes each Command through prlimit (resource limits) and landlock-restrict
// (filesystem and TCP allowlists), when those binaries are configured. Commands run in their own
// process group, so a timeout or cancellation kills the whole process tree, and they get only a
// minimal environment unless Config.InheritEnv is set.
//
// Without a Command.Dir, each run gets a temporary working directory that AutoCleanup removes; a
// directory supplied by the caller is never removed. The sandbox is only available on Linux; on
// other platforms commands run without restrictions, and Restrictions.Strict makes them fail.
package procrun
