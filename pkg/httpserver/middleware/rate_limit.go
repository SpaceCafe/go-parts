package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
)

const (
	DefaultConcurrentRequestLimit = 100
	DefaultRequestTimeout         = 30 * time.Second
	DefaultBucketCapacity         = 1000
	DefaultLeakRate               = 100
	DefaultLeakInterval           = time.Second
)

var (
	_ config.Defaultable = (*RateLimitConfig)(nil)
	_ config.Validatable = (*RateLimitConfig)(nil)

	ErrInvalidConcurrentRequestLimit = errors.New(
		"rate-limit: concurrent request limit must be non-negative",
	)
	ErrInvalidRequestTimeout = errors.New("rate-limit: request timeout must be positive")
	ErrInvalidBucketCapacity = errors.New(
		"rate-limit: bucket capacity must be positive",
	)
	ErrInvalidLeakRate = errors.New(
		"rate-limit: leak rate must be positive",
	)
	ErrInvalidLeakInterval = errors.New(
		"rate-limit: leak interval must be positive",
	)
)

// RateLimitConfig holds the configuration for RateLimit middleware.
type RateLimitConfig struct {
	// ConcurrentRequestLimit defines the maximum number of requests that can be processed concurrently.
	// If set to 0, there is no limit.
	ConcurrentRequestLimit int `json:"concurrentRequestLimit" yaml:"concurrentRequestLimit"`

	// RequestTimeout defines the maximum time a request can wait in the queue.
	// If a request waits longer than this, it will be rejected with a timeout error.
	RequestTimeout time.Duration `json:"requestTimeout" yaml:"requestTimeout"`

	// BucketCapacity defines the maximum number of requests that can accumulate in the bucket.
	// Once the bucket is full, additional requests are rejected immediately.
	BucketCapacity int `json:"bucketCapacity" yaml:"bucketCapacity"`

	// LeakRate defines how many request slots are freed per leak interval.
	// This determines the sustained request rate: LeakRate / LeakInterval = requests/second.
	LeakRate int `json:"leakRate" yaml:"leakRate"`

	// LeakInterval defines how often request slots are freed from the bucket.
	// Larger intervals with proportionally larger LeakRate reduce timer overhead.
	LeakInterval time.Duration `json:"leakInterval" yaml:"leakInterval"`
}

func (c *RateLimitConfig) SetDefaults() {
	c.ConcurrentRequestLimit = DefaultConcurrentRequestLimit
	c.RequestTimeout = DefaultRequestTimeout
	c.BucketCapacity = DefaultBucketCapacity
	c.LeakRate = DefaultLeakRate
	c.LeakInterval = DefaultLeakInterval
}

func (c *RateLimitConfig) Validate() error {
	if c.ConcurrentRequestLimit < 0 {
		return ErrInvalidConcurrentRequestLimit
	}

	if c.RequestTimeout <= 0 {
		return ErrInvalidRequestTimeout
	}

	if c.BucketCapacity <= 0 {
		return ErrInvalidBucketCapacity
	}

	if c.LeakRate <= 0 {
		return ErrInvalidLeakRate
	}

	if c.LeakInterval <= 0 {
		return ErrInvalidLeakInterval
	}

	return nil
}

// RateLimit returns a middleware for rate limiting incoming requests based on the provided configuration.
// It uses a leaky bucket algorithm for burst control and concurrent slot limiting for simultaneous requests.
func RateLimit(ctx context.Context, cfg *RateLimitConfig) httpserver.Middleware {
	bucket := NewLeakyBucket(cfg.BucketCapacity)
	bucket.StartLeaking(ctx, cfg.LeakRate, cfg.LeakInterval)

	concurrentSlots := NewLeakyBucket(cfg.ConcurrentRequestLimit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if bucket.TryAdd() {
				if concurrentSlots.TryAddWithTimeout(cfg.RequestTimeout) {
					defer concurrentSlots.Remove()

					next.ServeHTTP(resp, req)

					return
				}

				http.Error(
					resp,
					http.StatusText(http.StatusRequestTimeout),
					http.StatusRequestTimeout,
				)

				return
			}

			http.Error(
				resp,
				http.StatusText(http.StatusTooManyRequests),
				http.StatusTooManyRequests,
			)
		})
	}
}

// LeakyBucket is a limited-capacity channel that implements a leaky bucket algorithm for rate-limiting operations.
type LeakyBucket chan struct{}

func NewLeakyBucket(capacity int) LeakyBucket {
	if capacity <= 0 {
		return nil
	}

	return make(chan struct{}, capacity)
}

// Remove removes a single element from the bucket if available, otherwise does nothing.
func (b LeakyBucket) Remove() {
	if b == nil {
		return
	}

	select {
	case <-b:
	default:
	}
}

// StartLeaking begins removing elements from the bucket at a steady rate defined by the interval and rate parameters.
func (b LeakyBucket) StartLeaking(ctx context.Context, rate int, interval time.Duration) {
	if b == nil || rate <= 0 || interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for range rate {
					select {
					case <-b:
					default:
						break
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// TryAdd attempts to add an element to the bucket. Returns true if successful, or false if the bucket is full.
func (b LeakyBucket) TryAdd() bool {
	if b == nil {
		return true
	}

	select {
	case b <- struct{}{}:
		return true
	default:
		return false
	}
}

// TryAddWithTimeout attempts to add an element to the bucket within the specified timeout duration.
// Returns true if successful.
func (b LeakyBucket) TryAddWithTimeout(timeout time.Duration) bool {
	if b == nil {
		return true
	}

	select {
	case b <- struct{}{}:
		return true
	case <-time.After(timeout):
		return false
	}
}
