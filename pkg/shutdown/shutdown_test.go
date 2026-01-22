package shutdown_test

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/shutdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ErrMock = errors.New("stop error")

type MockService struct {
	StopTimeout time.Duration
	StopCalled  chan bool
	ReturnError error
}

func (m *MockService) Start(_ context.Context) error {
	m.StopCalled = make(chan bool, 1)

	return nil
}

func (m *MockService) Stop(_ context.Context) error {
	m.StopCalled <- true

	<-time.After(m.StopTimeout)

	return m.ReturnError
}

func sendSignal(t *testing.T, signal os.Signal) {
	t.Helper()

	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	err = p.Signal(signal)
	require.NoError(t, err)
}

//nolint:paralleltest // This test is not safe to run in parallel.
func TestShutdown_Track(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *shutdown.Config
		arg     any
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "timeout greater than stop timeout",
			cfg:     &shutdown.Config{Timeout: time.Second * 2, Force: true},
			arg:     &MockService{StopTimeout: time.Second},
			wantErr: assert.NoError,
		},
		{
			name:    "timeout less than stop timeout",
			cfg:     &shutdown.Config{Timeout: 0, Force: true},
			arg:     &MockService{StopTimeout: time.Second},
			wantErr: assert.NoError,
		},
		{
			name:    "timeout greater than stop timeout without force",
			cfg:     &shutdown.Config{Timeout: time.Second * 2, Force: false},
			arg:     &MockService{StopTimeout: time.Second},
			wantErr: assert.NoError,
		},
		{
			name:    "timeout less than stop timeout without force",
			cfg:     &shutdown.Config{Timeout: 0, Force: false},
			arg:     &MockService{StopTimeout: time.Second},
			wantErr: assert.NoError,
		},
		{
			name:    "return error on stop",
			cfg:     &shutdown.Config{Timeout: time.Second, Force: false},
			arg:     &MockService{ReturnError: ErrMock},
			wantErr: assert.NoError,
		},
		{
			name:    "not trackable",
			cfg:     &shutdown.Config{Timeout: time.Second * 2, Force: true},
			arg:     nil,
			wantErr: assert.NoError,
		},
		{
			name:    "not trackable struct",
			cfg:     &shutdown.Config{Timeout: time.Second * 2, Force: true},
			arg:     &struct{}{},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := shutdown.New(tt.cfg)

			exitCh := make(chan int, 1)
			obj.ExitFn = func(code int) {
				exitCh <- code
			}

			err := obj.Track(tt.arg)
			tt.wantErr(t, err)

			sendSignal(t, syscall.SIGTERM)

			select {
			case code := <-exitCh:
				assert.Equal(t, shutdown.ExitCodeSigTerm, code)
				t.Log("shutdown completed with exit code")
			case <-obj.Done():
				t.Log("shutdown completed without exit code")
			case <-time.After(tt.cfg.Timeout + time.Second):
				t.Fatal("timeout reached")
			}

			if v, ok := tt.arg.(*MockService); ok {
				assert.True(t, <-v.StopCalled)
			}
		})
	}
}

//nolint:paralleltest // This test is not safe to run in parallel.
func TestShutdown_Integration(t *testing.T) {
	obj := shutdown.New(&shutdown.Config{Timeout: time.Second * 2, Force: false})
	require.NotNil(t, obj.Context())
	assert.Implements(t, (*context.Context)(nil), obj.Context())

	service := &MockService{StopTimeout: time.Second}
	err := obj.Track(service)
	require.NoError(t, err)

	testValue := false

	err = obj.Go(func(_ context.Context) { testValue = true })
	require.NoError(t, err)

	sendSignal(t, syscall.SIGUSR1)
	<-time.After(time.Second * 3)

	err = obj.Track(service)
	require.ErrorIs(t, shutdown.ErrContextCancelled, err)

	err = obj.Go(func(_ context.Context) {})
	require.ErrorIs(t, shutdown.ErrContextCancelled, err)

	start := time.Now()

	go obj.Shutdown()

	obj.Wait()

	elapsed := time.Since(start)

	assert.True(t, <-service.StopCalled)
	assert.True(t, testValue)
	assert.Lessf(t, elapsed, time.Second, "shutdown took too long: %obj", elapsed)
}
