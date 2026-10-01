// Landlock-restrict runs a command with Landlock filesystem and TCP restrictions applied.
//
// Usage:
//
//	landlock-restrict [flags] -- command [args...]
//
// Path flags (-ro.dir, -ro.file, -rw.dir, -rw.file) act together: giving any of them restricts
// all filesystem access to the listed paths. TCP flags (-tcp.bind, -tcp.connect) are independent;
// a TCP direction that is never named stays unrestricted. Flags may be repeated or take a list
// split like $PATH. An empty value enforces the restriction with nothing allowed.
//
// Without -strict, the command runs with whatever the kernel supports and a warning is printed
// when that is less than requested. With -strict, it fails instead of running a requested kind of
// restriction unenforced (filesystem rules need Landlock ABI v1, TCP rules need v4). With -quiet,
// the warning is suppressed, so the command's stderr carries only its own output. Errors that
// prevent the command from running are still reported.
//
// Exit codes follow env(1): 125 when the restrictions cannot be applied, 126 when the command
// cannot be executed, 127 when it cannot be found, and 2 for a malformed command line.
package main
