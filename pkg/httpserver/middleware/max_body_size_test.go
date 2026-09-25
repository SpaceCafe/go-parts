package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

func TestMaxBodySize(t *testing.T) {
	t.Parallel()

	// jsonBody returns a JSON string document of exactly size bytes.
	jsonBody := func(size int) string { return `"` + strings.Repeat("a", size-2) + `"` }

	tests := []struct {
		name       string
		chain      []httpserver.Middleware
		bodySize   int
		wantStatus int
	}{
		{
			name:       "body within limit",
			chain:      []httpserver.Middleware{middleware.MaxBodySize(64)},
			bodySize:   64,
			wantStatus: http.StatusOK,
		},
		{
			name:       "body over limit",
			chain:      []httpserver.Middleware{middleware.MaxBodySize(64)},
			bodySize:   65,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "inner limit raises outer limit",
			chain: []httpserver.Middleware{
				middleware.MaxBodySize(16),
				middleware.MaxBodySize(128),
			},
			bodySize:   100,
			wantStatus: http.StatusOK,
		},
		{
			name: "inner limit lowers outer limit",
			chain: []httpserver.Middleware{
				middleware.MaxBodySize(128),
				middleware.MaxBodySize(16),
			},
			bodySize:   100,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "zero limit removes outer limit",
			chain: []httpserver.Middleware{
				middleware.MaxBodySize(16),
				middleware.MaxBodySize(0),
			},
			bodySize:   100,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handler http.Handler = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				var target string

				err := httpserver.GetJSONBody(req, &target)
				if errors.Is(err, httpserver.ErrRequestTooLarge) {
					resp.WriteHeader(http.StatusRequestEntityTooLarge)

					return
				}

				assert.NoError(t, err)
				resp.WriteHeader(http.StatusOK)
			})

			for _, mw := range slices.Backward(tt.chain) {
				handler = mw(handler)
			}

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/",
				strings.NewReader(jsonBody(tt.bodySize)),
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
