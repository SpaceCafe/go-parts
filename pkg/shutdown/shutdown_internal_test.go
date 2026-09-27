//go:build unix

package shutdown

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Sends SIGTERM to the test process.
func TestShutdown_SignalHandlerReleasedAfterFirstSignal(t *testing.T) {
	// guard keeps the test process alive once the instance under test stops listening.
	guard := make(chan os.Signal, 4)
	signal.Notify(guard, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(guard) })

	obj := New(&Config{Timeout: time.Second, Force: false})

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case <-obj.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("first signal did not shut down")
	}

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	time.Sleep(100 * time.Millisecond)

	assert.Empty(t, obj.signalCh, "instance must not receive signals after shutting down")
}
