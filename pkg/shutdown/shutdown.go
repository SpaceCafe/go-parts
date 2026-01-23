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
)

var ErrContextCancelled = errors.New("shutdown: context cancelled")

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
}

// New creates a new Shutdown instance with the provided configuration.
func New(cfg *Config) *Shutdown {
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

	// Listen to interrupt, termination, and user signals.
	signal.Notify(obj.signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		defer obj.Shutdown()

		for {
			sig := <-obj.signalCh
			if sig == syscall.SIGUSR1 {
				obj.Drain()
			} else {
				break
			}
		}
	}()

	return obj
}

// Context returns the context but does not track the goroutine.
// This is useful when you need the context outside the termination flow.
func (s *Shutdown) Context() context.Context {
	return s.runtimeCtx
}

// Done returns a channel which is closed when the shutdown process is complete.
func (s *Shutdown) Done() <-chan struct{} {
	<-s.runtimeCtx.Done()

	return s.shutdownCtx.Done()
}

// Drain initiates a graceful drain without termination.
// Workers are stopped gracefully, but the process stays alive.
// Use this to stop accepting new connections or long-running tasks.
func (s *Shutdown) Drain() {
	s.Log.Info("shutdown: initializing drain")
	s.cancelRuntimeFn()

	go s.observeShutdown(nil)
}

// Go calls the given task in a new goroutine and adds that task to the waitGroup.
// When the task returns, it's removed from the waitGroup.
// Use this for background tasks that should be tracked for graceful shutdown.
func (s *Shutdown) Go(task func(context.Context)) error {
	if s.runtimeCtx.Err() != nil {
		return ErrContextCancelled
	}

	s.waitGroup.Add(1)
	s.Log.Debug("shutdown: starting task")

	go func() {
		defer s.waitGroup.Done()

		task(s.runtimeCtx)
	}()

	return nil
}

// Shutdown initiates a graceful shutdown manually without waiting for a signal.
// This is useful for programmatic shutdown scenarios.
func (s *Shutdown) Shutdown() {
	s.Log.Info("shutdown: initializing shutdown")
	s.cancelRuntimeFn()

	go s.observeShutdown(s.cancelShutdownFn)

	select {
	case <-s.shutdownCtx.Done():
		s.Log.Info("shutdown: shutdown gracefully completed")
	case <-time.After(s.cfg.Timeout):
		s.cancelShutdownFn()
		s.Log.Error("shutdown: shutdown timed out")
	}

	if s.cfg.Force {
		s.Log.Info("shutdown: shutting down forcefully")
		s.ExitFn(ExitCodeSigTerm)
	}
}

// Track initiates a trackable entity, adding it to the wait group and invoking its Start method with the given context.
func (s *Shutdown) Track(service any) error {
	if s.runtimeCtx.Err() != nil {
		return ErrContextCancelled
	}

	s.waitGroup.Add(1)

	if service == nil {
		return nil
	}

	if trackable, ok := service.(Trackable); ok {
		go func() {
			defer s.waitGroup.Done()

			<-s.runtimeCtx.Done()

			err := trackable.Stop(s.shutdownCtx)
			if err != nil {
				s.Log.Error("shutdown: failed to stop service", "error", err)
			}
		}()

		err := trackable.Start(s.runtimeCtx)
		if err != nil {
			return fmt.Errorf("shutdown: starting service service: %w", err)
		}

		s.Log.Debug("shutdown: starting service")
	}

	return nil
}

// Wait blocks until all tracked goroutines have finished.
// Use this function at the end of the main function.
func (s *Shutdown) Wait() {
	<-s.runtimeCtx.Done()
	<-s.shutdownCtx.Done()
}

func (s *Shutdown) observeShutdown(callback func()) {
	s.waitGroup.Wait()
	s.Log.Info("shutdown: all tasks completed")

	if callback != nil {
		callback()
	}
}
