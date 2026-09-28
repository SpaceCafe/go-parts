package shutdown_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/shutdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockService struct {
	ReturnError error
	StopCalled  chan bool
	StopTimeout time.Duration
}

func (m *mockService) Start(_ context.Context) error {
	m.StopCalled = make(chan bool, 1)

	return nil
}

func (m *mockService) Stop(_ context.Context) error {
	m.StopCalled <- true

	<-time.After(m.StopTimeout)

	return m.ReturnError
}

//nolint:paralleltest // Other tests send SIGTERM to the process, which every instance receives.
func TestShutdown_Track_RejectedServiceDoesNotBlockShutdown(t *testing.T) {
	obj := shutdown.New(&shutdown.Config{Timeout: 5 * time.Second, Force: false})

	require.ErrorIs(t, obj.Track(nil), shutdown.ErrNotTrackable)
	require.ErrorIs(t, obj.Track(&struct{}{}), shutdown.ErrNotTrackable)

	begin := time.Now()

	obj.Shutdown()

	assert.Less(t, time.Since(begin), time.Second, "shutdown must not wait for the timeout")
}

//nolint:paralleltest // Other tests send SIGTERM to the process, which every instance receives.
func TestShutdown_Done_DoesNotBlock(t *testing.T) {
	obj := shutdown.New(&shutdown.Config{Timeout: time.Second, Force: false})

	returned := make(chan (<-chan struct{}), 1)

	go func() { returned <- obj.Done() }()

	var done <-chan struct{}

	select {
	case done = <-returned:
	case <-time.After(time.Second):
		t.Fatal("Done blocked on a running instance")
	}

	select {
	case <-done:
		t.Fatal("Done closed before shutdown")
	default:
	}

	obj.Shutdown()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Done not closed after shutdown")
	}
}

//nolint:paralleltest // Other tests send SIGTERM to the process, which every instance receives.
func TestShutdown_ForceExitsOnlyOnTimeout(t *testing.T) {
	tests := []struct {
		service  *mockService
		name     string
		wantExit []int
	}{
		{name: "clean shutdown does not exit", service: &mockService{}, wantExit: nil},
		{
			name:     "timed out shutdown exits",
			service:  &mockService{StopTimeout: time.Second},
			wantExit: []int{shutdown.ExitCodeTimeout},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := shutdown.New(&shutdown.Config{Timeout: 100 * time.Millisecond, Force: true})

			var exits []int

			obj.ExitFn = func(code int) { exits = append(exits, code) }

			require.NoError(t, obj.Track(tt.service))

			obj.Shutdown()

			assert.Equal(t, tt.wantExit, exits)
		})
	}
}

//nolint:paralleltest // Other tests send SIGTERM to the process, which every instance receives.
func TestShutdown_Go_ConcurrentWithShutdown(t *testing.T) {
	for range 50 {
		obj := shutdown.New(&shutdown.Config{Timeout: time.Second, Force: false})

		var (
			accepted atomic.Int64
			finished atomic.Int64
			starters sync.WaitGroup
		)

		for range 20 {
			starters.Go(func() {
				err := obj.Go(func(ctx context.Context) {
					<-ctx.Done()
					finished.Add(1)
				})
				if err == nil {
					accepted.Add(1)
				}
			})
		}

		obj.Shutdown()
		starters.Wait()

		// Every task that Go accepted must have been waited for by the graceful shutdown.
		assert.Equal(t, accepted.Load(), finished.Load())
	}
}

// countingLogger counts Info messages by text. Observer goroutines log concurrently, hence mu.
type countingLogger struct {
	info map[string]int
	mu   sync.Mutex
}

func (l *countingLogger) Debug(string, ...any) {}
func (l *countingLogger) Error(string, ...any) {}

func (l *countingLogger) Info(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.info[msg]++
}

func (l *countingLogger) Warn(string, ...any) {}

func (l *countingLogger) count(msg string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.info[msg]
}

func TestShutdown_DrainOnce(t *testing.T) {
	t.Parallel()

	obj := shutdown.New(&shutdown.Config{Timeout: time.Second})
	logger := &countingLogger{info: map[string]int{}}
	obj.Log = logger

	for range 3 {
		obj.Drain()
	}

	assert.Equal(t, 1, logger.count("shutdown: initializing drain"))
	require.ErrorIs(t, obj.Context().Err(), context.Canceled)
}
