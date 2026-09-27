// Package shutdown coordinates graceful shutdown of background goroutines and services.
//
// Shutdown listens for SIGINT and SIGTERM (and SIGUSR1 for Drain on Unix). Go runs tracked
// goroutines and Track starts Trackable services; on shutdown their context is cancelled, services
// are stopped, and Shutdown waits up to Config.Timeout for everything to finish. With Config.Force,
// the process exits only when that timeout is exceeded. A second signal during a hung shutdown
// terminates the process.
package shutdown
