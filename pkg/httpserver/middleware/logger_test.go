package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

// queryLogger keeps the "query" argument of the last Info call.
type queryLogger struct {
	query any
}

func (l *queryLogger) Debug(string, ...any) {}
func (l *queryLogger) Error(string, ...any) {}

func (l *queryLogger) Info(_ string, args ...any) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "query" {
			l.query = args[i+1]
		}
	}
}

func (l *queryLogger) Warn(string, ...any) {}

func TestLogger_Query(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want any
		name string
		opts []middleware.LoggerOption
	}{
		{
			name: "keys only by default",
			want: []string{"api_key", "page"},
		},
		{
			name: "full query with WithQueryValues",
			opts: []middleware.LoggerOption{middleware.WithQueryValues()},
			want: url.Values{"api_key": {"s3cr3t"}, "page": {"2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &queryLogger{}
			handler := middleware.Logger(logger, tt.opts...)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/?page=2&api_key=s3cr3t",
				http.NoBody,
			)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.want, logger.query)
		})
	}
}
