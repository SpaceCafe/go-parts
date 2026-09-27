//go:build !unix

package shutdown

import (
	"os"
	"os/signal"
	"syscall"
)

// notifySignals registers ch for the interrupt and termination signals. There is no SIGUSR1 on
// this platform, so draining is only available through Drain.
func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

// isDrainSignal reports whether sig requests a drain instead of a shutdown. No signal does on this
// platform.
func isDrainSignal(os.Signal) bool {
	return false
}
