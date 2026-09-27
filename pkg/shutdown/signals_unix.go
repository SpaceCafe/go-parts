//go:build unix

package shutdown

import (
	"os"
	"os/signal"
	"syscall"
)

// notifySignals registers ch for the interrupt and termination signals, plus SIGUSR1 for draining.
func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
}

// isDrainSignal reports whether sig requests a drain instead of a shutdown.
func isDrainSignal(sig os.Signal) bool {
	return sig == syscall.SIGUSR1
}
