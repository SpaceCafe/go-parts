package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spacecafe/go-parts/pkg/log"
)

const (
	// ExitCodeSigTerm is the exit status code for SIGTERM,
	// indicating the container received a SIGTERM by the underlying operating system.
	ExitCodeSigTerm = 128 + int(syscall.SIGTERM) // equals 143

	// ExitCodeTimeout is the exit status code when a programmatic Shutdown timed out.
	ExitCodeTimeout = 1
)

var (
	ErrContextCancelled = errors.New("shutdown: context cancelled")
	ErrNotTrackable     = errors.New("shutdown: service does not implement Trackable")
)

// Trackable represents an interface for managing the lifecycle of a trackable goroutine.
type Trackable interface {
	// Start begins the trackable goroutine with a given context.
	Start(ctx context.Context) error

	// Stop halts the tracked goroutine.
	Stop(ctx context.Context) error
}

// Shutdown is a struct that manages context cancellation and synchronization.
type Shutdown struct {
	// runtimeCtx is the context for managing cancellation.
	//nolint:containedctx // Shutdown is an extension of context.Context to provide additional functionality.
	runtimeCtx context.Context

	// shutdownCtx is the context for managing shutdown operations.
	//nolint:containedctx // Shutdown is an extension of context.Context to provide additional functionality.
	shutdownCtx context.Context

	// Log is the logger instance.
	Log log.Logger

	// cfg holds configuration settings.
	cfg *Config

	// ExitFn allows overriding os.Exit for testing
	ExitFn func(int)

	// cancelRuntimeFn is the function to cancel the runtime context.
	cancelRuntimeFn context.CancelFunc

	// cancelShutdownFn is the function to cancel the shutdown context.
	cancelShutdownFn context.CancelFunc

	// signalCh is a channel used to receive operating system signals
	// for handling graceful shutdowns or specific behaviors.
	signalCh chan os.Signal

	// waitGroup is used to synchronize and wait for the completion of multiple goroutines.
	waitGroup sync.WaitGroup

	// mu serializes adding to waitGroup with cancelling the runtime context. Once the context is
	// cancelled under mu, no further Add can happen, so Add never races with Wait.
	mu sync.Mutex
}

// New creates a new Shutdown instance with the provided configuration.
func New(cfg *Config) *Shutdown {
	if cfg == nil {
		cfg = &Config{}
		cfg.SetDefaults()
	}

	runtimeCtx, cancelRuntimeFn := context.WithCancel(context.Background())
	shutdownCtx, cancelShutdownFn := context.WithCancel(context.Background())
	obj := &Shutdown{
		runtimeCtx:       runtimeCtx,
		shutdownCtx:      shutdownCtx,
		Log:              slog.Default(),
		cfg:              cfg,
		ExitFn:           os.Exit,
		cancelRuntimeFn:  cancelRuntimeFn,
		cancelShutdownFn: cancelShutdownFn,
		signalCh:         make(chan os.Signal, 1),
	}

	// Listen to interrupt, termination, and (where available) user signals.
	notifySignals(obj.signalCh)

	go obj.handleSignals()

	return obj
}

// Context returns the context but does not track the goroutine.
// This is useful when you need the context outside the termination flow.
func (s *Shutdown) Context() context.Context {
	return s.runtimeCtx
}

// Done returns a channel which is closed when the shutdown process is complete. It returns
// immediately, like context.Context.Done. The shutdown context is only cancelled by Shutdown after
// the runtime context, so the channel never closes while the instance is still running.
func (s *Shutdown) Done() <-chan struct{} {
	return s.shutdownCtx.Done()
}

// Drain initiates a graceful drain without termination.
// Workers are stopped gracefully, but the process stays alive.
// Use this to stop accepting new connections or long-running tasks.
func (s *Shutdown) Drain() {
	s.Log.Info("shutdown: initializing drain")
	s.cancelRuntime()

	go s.observeShutdown(nil)
}

// Go calls the given task in a new goroutine and adds that task to the waitGroup.
// When the task returns, it's removed from the waitGroup.
// Use this for background tasks that should be tracked for graceful shutdown.
func (s *Shutdown) Go(task func(context.Context)) error {
	err := s.goTracked(func() { task(s.runtimeCtx) })
	if err != nil {
		return err
	}

	s.Log.Debug("shutdown: starting task")

	return nil
}

// Shutdown initiates a graceful shutdown manually without waiting for a signal.
// This is useful for programmatic shutdown scenarios.
func (s *Shutdown) Shutdown() {
	s.shutdown(false)
}

// Track initiates a trackable entity, adding it to the wait group and invoking its Start method with the given context.
// A service that is nil or does not implement Trackable is rejected with ErrNotTrackable and is not
// added to the wait group, so it cannot block shutdown.
func (s *Shutdown) Track(service any) error {
	trackable, ok := service.(Trackable)
	if !ok {
		return fmt.Errorf("%w: %T", ErrNotTrackable, service)
	}

	// Reserve the wait group slot before Start, so a shutdown that begins during Start still waits
	// for the service, but start the goroutine that calls Stop only after Start has returned: Stop
	// must never run concurrently with Start.
	err := s.addTracked()
	if err != nil {
		return err
	}

	err = trackable.Start(s.runtimeCtx)
	if err != nil {
		s.waitGroup.Done()

		return fmt.Errorf("shutdown: starting service: %w", err)
	}

	go func() {
		defer s.waitGroup.Done()

		<-s.runtimeCtx.Done()

		err := trackable.Stop(s.shutdownCtx)
		if err != nil {
			s.Log.Error("shutdown: failed to stop service", "error", err)
		}
	}()

	s.Log.Debug("shutdown: starting service")

	return nil
}

// Wait blocks until all tracked goroutines have finished.
// Use this function at the end of the main function.
func (s *Shutdown) Wait() {
	<-s.runtimeCtx.Done()
	<-s.shutdownCtx.Done()
}

// addTracked adds one entry to the wait group, unless the runtime context is already cancelled.
// The check and the Add happen under mu, which cancelRuntime also holds, so a shutdown cannot start
// waiting between them. The caller must call waitGroup.Done exactly once.
func (s *Shutdown) addTracked() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runtimeCtx.Err() != nil {
		return ErrContextCancelled
	}

	s.waitGroup.Add(1)

	return nil
}

// cancelRuntime cancels the runtime context while holding mu, see addTracked.
func (s *Shutdown) cancelRuntime() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancelRuntimeFn()
}

// goTracked runs task in a goroutine tracked by the wait group, see addTracked.
func (s *Shutdown) goTracked(task func()) error {
	err := s.addTracked()
	if err != nil {
		return err
	}

	go func() {
		defer s.waitGroup.Done()

		task()
	}()

	return nil
}

// handleSignals drains on SIGUSR1 and shuts down on the first interrupt or termination signal. It
// then stops receiving signals, which restores their default behavior: a second Ctrl+C during a
// hung shutdown terminates the process. It also stops once a programmatic shutdown completes, so
// neither the goroutine nor the signal registration outlives the instance.
func (s *Shutdown) handleSignals() {
	for {
		select {
		case sig := <-s.signalCh:
			if isDrainSignal(sig) {
				s.Drain()

				continue
			}

			signal.Stop(s.signalCh)
			s.shutdown(true)

			return
		case <-s.shutdownCtx.Done():
			signal.Stop(s.signalCh)

			return
		}
	}
}

func (s *Shutdown) observeShutdown(callback func()) {
	s.waitGroup.Wait()
	s.Log.Info("shutdown: all tasks completed")

	if callback != nil {
		callback()
	}
}

// shutdown cancels the runtime context and waits for tracked services to stop. With Config.Force,
// it exits the process only when the graceful shutdown timed out: with ExitCodeSigTerm when a
// signal triggered it, otherwise with ExitCodeTimeout. A clean shutdown always returns, so main can
// finish and run its deferred functions.
func (s *Shutdown) shutdown(signaled bool) {
	s.Log.Info("shutdown: initializing shutdown")
	s.cancelRuntime()

	go s.observeShutdown(s.cancelShutdownFn)

	select {
	case <-s.shutdownCtx.Done():
		s.Log.Info("shutdown: shutdown gracefully completed")

		return
	case <-time.After(s.cfg.Timeout):
		s.cancelShutdownFn()
		s.Log.Error("shutdown: shutdown timed out")
	}

	if !s.cfg.Force {
		return
	}

	s.Log.Info("shutdown: shutting down forcefully")

	if signaled {
		s.ExitFn(ExitCodeSigTerm)
	} else {
		s.ExitFn(ExitCodeTimeout)
	}
}
