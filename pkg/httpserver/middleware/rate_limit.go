package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/validate"
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

// SetDefaults applies the default limits, yielding a sustained rate of DefaultLeakRate requests per
// DefaultLeakInterval on top of the burst and concurrency caps.
func (c *RateLimitConfig) SetDefaults() {
	c.ConcurrentRequestLimit = DefaultConcurrentRequestLimit
	c.RequestTimeout = DefaultRequestTimeout
	c.BucketCapacity = DefaultBucketCapacity
	c.LeakRate = DefaultLeakRate
	c.LeakInterval = DefaultLeakInterval
}

// Validate ensures every limit is within range. The concurrent request limit may be zero to disable
// the concurrency cap, while the remaining values must be positive to keep the bucket leaking.
func (c *RateLimitConfig) Validate() error {
	return errors.Join(
		validate.Validate(
			"concurrent request limit",
			c.ConcurrentRequestLimit,
			validate.NonNegative,
		),
		validate.Validate("request timeout", c.RequestTimeout, validate.Positive),
		validate.Validate("bucket capacity", c.BucketCapacity, validate.Positive),
		validate.Validate("leak rate", c.LeakRate, validate.Positive),
		validate.Validate("leak interval", c.LeakInterval, validate.Positive),
	)
}

// RateLimit returns middleware for rate-limiting incoming requests based on the provided configuration.
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

				httpserver.Abort(resp, req, http.StatusRequestTimeout, nil)

				return
			}

			httpserver.Abort(resp, req, http.StatusTooManyRequests, nil)
		})
	}
}

// LeakyBucket is a limited-capacity channel that implements a leaky bucket algorithm for rate-limiting operations.
type LeakyBucket chan struct{}

// NewLeakyBucket returns a bucket with the given capacity. A non-positive capacity yields a nil
// bucket, which the methods treat as unlimited so the limit can be disabled without special cases.
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
