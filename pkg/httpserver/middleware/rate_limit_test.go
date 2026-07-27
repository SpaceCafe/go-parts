package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg          func(*middleware.RateLimitConfig)
		name         string
		wantStatus   []int
		requestCount int
		requestDelay time.Duration
	}{
		{
			name: "allows requests within token bucket limit",
			cfg: func(cfg *middleware.RateLimitConfig) {
				cfg.BucketCapacity = 5
				cfg.LeakRate = 0
				cfg.LeakInterval = time.Minute
				cfg.ConcurrentRequestLimit = 10
				cfg.RequestTimeout = time.Second
			},
			requestCount: 5,
			wantStatus: []int{
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
			},
		},
		{
			name: "rejects requests exceeding token bucket",
			cfg: func(cfg *middleware.RateLimitConfig) {
				cfg.BucketCapacity = 2
				cfg.LeakRate = 0
				cfg.LeakInterval = time.Minute
				cfg.ConcurrentRequestLimit = 10
				cfg.RequestTimeout = time.Second
			},
			requestCount: 4,
			wantStatus: []int{
				http.StatusOK,
				http.StatusOK,
				http.StatusTooManyRequests,
				http.StatusTooManyRequests,
			},
		},
		{
			name: "respects concurrent request limit",
			cfg: func(cfg *middleware.RateLimitConfig) {
				cfg.BucketCapacity = 10
				cfg.LeakRate = 10
				cfg.LeakInterval = time.Millisecond
				cfg.ConcurrentRequestLimit = 2
				cfg.RequestTimeout = 10 * time.Millisecond
			},
			requestCount: 3,
			requestDelay: 50 * time.Millisecond,
			wantStatus:   []int{http.StatusOK, http.StatusOK, http.StatusRequestTimeout},
		},
		{
			name: "no concurrent limit when set to 0",
			cfg: func(cfg *middleware.RateLimitConfig) {
				cfg.BucketCapacity = 10
				cfg.LeakRate = 10
				cfg.LeakInterval = time.Millisecond
				cfg.ConcurrentRequestLimit = 0
				cfg.RequestTimeout = time.Second
			},
			requestCount: 5,
			wantStatus: []int{
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
			},
		},
		{
			name: "no burst limit when set to 0",
			cfg: func(cfg *middleware.RateLimitConfig) {
				cfg.BucketCapacity = 0
				cfg.LeakRate = 0
				cfg.LeakInterval = time.Second
				cfg.ConcurrentRequestLimit = 0
				cfg.RequestTimeout = time.Second
			},
			requestCount: 5,
			wantStatus: []int{
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
				http.StatusOK,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.RateLimitConfig{}
			cfg.SetDefaults()
			tt.cfg(cfg)

			ctx := t.Context()

			handler := middleware.RateLimit(ctx, cfg)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if tt.requestDelay > 0 {
						time.Sleep(tt.requestDelay)
					}

					w.WriteHeader(http.StatusOK)
				}),
			)

			// For concurrent limit test, run requests concurrently
			if tt.requestDelay > 0 {
				var waitGroup sync.WaitGroup

				statusCodes := make([]int, tt.requestCount)

				for i := range tt.requestCount {
					waitGroup.Add(1)

					go func(idx int) {
						defer waitGroup.Done()

						req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, req)
						statusCodes[idx] = rec.Code
					}(i)
				}

				waitGroup.Wait()
			} else {
				// For token bucket tests, run requests sequentially
				statusCodes := make([]int, 0, tt.requestCount)

				for range tt.requestCount {
					req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					statusCodes = append(statusCodes, rec.Code)
				}

				assert.Equal(t, tt.wantStatus, statusCodes, "unexpected status codes")
			}
		})
	}
}
