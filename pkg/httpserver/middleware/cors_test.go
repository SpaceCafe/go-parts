package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg                *middleware.CORSConfig
		expectedHeaders    map[string]string
		requestHeaders     map[string]string
		name               string
		requestOrigin      string
		requestMethod      string
		expectedStatusCode int
	}{
		{
			name: "valid origin, simple GET request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "https://example.com",
			requestMethod:      http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://example.com",
			},
		},
		{
			name: "invalid origin, simple GET request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "https://notallowed.com",
			requestMethod:      http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedHeaders:    map[string]string{}, // No CORS headers expected
		},
		{
			name: "wildcard origin, simple GET request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "https://anysite.com",
			requestMethod:      http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
		{
			name: "valid origin, preflight OPTIONS request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet, http.MethodPost},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
				MaxAge:         3600,
			},
			requestOrigin:      "https://example.com",
			requestMethod:      http.MethodOptions,
			requestHeaders:     map[string]string{"Access-Control-Request-Method": "POST"},
			expectedStatusCode: http.StatusNoContent,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
				"Access-Control-Max-Age":       "3600",
			},
		},
		{
			name: "invalid origin, preflight OPTIONS request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet, http.MethodPost},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "https://notallowed.com",
			requestMethod:      http.MethodOptions,
			requestHeaders:     map[string]string{"Access-Control-Request-Method": "POST"},
			expectedStatusCode: http.StatusForbidden,
			expectedHeaders:    map[string]string{}, // No CORS headers expected
		},
		{
			name: "OPTIONS without request method reaches handler",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "https://example.com",
			requestMethod:      http.MethodOptions,
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://example.com",
			},
		},
		{
			name: "OPTIONS without origin reaches handler",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestMethod:      http.MethodOptions,
			requestHeaders:     map[string]string{"Access-Control-Request-Method": "POST"},
			expectedStatusCode: http.StatusOK,
			expectedHeaders:    map[string]string{},
		},
		{
			name: "credentials support enabled",
			cfg: &middleware.CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{http.MethodGet},
				AllowedHeaders:   []string{},
				AllowCredentials: true,
			},
			requestOrigin:      "https://example.com",
			requestMethod:      http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
			},
		},
		{
			name: "no origin header in request",
			cfg: &middleware.CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{http.MethodGet},
				AllowedHeaders: []string{},
			},
			requestOrigin:      "",
			requestMethod:      http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedHeaders:    map[string]string{}, // No origin means no CORS headers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.CORS(
				tt.cfg,
			)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequestWithContext(
				t.Context(),
				tt.requestMethod,
				"http://localhost",
				http.NoBody,
			)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			res := rec.Result()

			defer func() {
				_ = res.Body.Close()
			}()

			assert.Equal(t, tt.expectedStatusCode, res.StatusCode, "unexpected status code")

			for key, value := range tt.expectedHeaders {
				got := res.Header.Get(key)
				assert.Equal(t, value, got, "header mismatch for %s", key)
			}

			for key := range res.Header {
				_, ok := tt.expectedHeaders[key]
				assert.False(
					t,
					!ok && strings.HasPrefix(key, "Access-Control-"),
					"unexpected header: %s",
					key,
				)
			}
		})
	}
}

func TestCORS_Vary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		origins  []string
		method   string
		origin   string
		wantVary []string
	}{
		{
			name:     "allowed origin",
			origins:  []string{"https://example.com"},
			method:   http.MethodGet,
			origin:   "https://example.com",
			wantVary: []string{"Origin"},
		},
		{
			name:     "disallowed origin",
			origins:  []string{"https://example.com"},
			method:   http.MethodGet,
			origin:   "https://other.com",
			wantVary: []string{"Origin"},
		},
		{
			name:     "no origin",
			origins:  []string{"https://example.com"},
			method:   http.MethodGet,
			wantVary: []string{"Origin"},
		},
		{
			name:     "wildcard origin",
			origins:  []string{"*"},
			method:   http.MethodGet,
			origin:   "https://example.com",
			wantVary: nil,
		},
		{
			name:    "preflight",
			origins: []string{"https://example.com"},
			method:  http.MethodOptions,
			origin:  "https://example.com",
			wantVary: []string{
				"Origin",
				"Access-Control-Request-Method",
				"Access-Control-Request-Headers",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.CORSConfig{}
			cfg.SetDefaults()
			cfg.AllowedOrigins = tt.origins

			handler := middleware.CORS(
				cfg,
			)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequestWithContext(
				t.Context(),
				tt.method,
				"http://localhost",
				http.NoBody,
			)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantVary, rec.Header().Values("Vary"))
		})
	}
}

func TestCORSConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg     func(*middleware.CORSConfig)
		wantErr error
		name    string
	}{
		{name: "defaults", cfg: func(*middleware.CORSConfig) {}},
		{
			name: "credentials with explicit origin",
			cfg: func(cfg *middleware.CORSConfig) {
				cfg.AllowedOrigins = []string{"https://example.com"}
				cfg.AllowCredentials = true
			},
		},
		{
			name:    "credentials with wildcard origin",
			cfg:     func(cfg *middleware.CORSConfig) { cfg.AllowCredentials = true },
			wantErr: middleware.ErrWildcardCredentials,
		},
		{
			name:    "empty allowed origins",
			cfg:     func(cfg *middleware.CORSConfig) { cfg.AllowedOrigins = []string{} },
			wantErr: middleware.ErrMissingAllowedOrigins,
		},
		{
			name:    "empty allowed methods",
			cfg:     func(cfg *middleware.CORSConfig) { cfg.AllowedMethods = []string{} },
			wantErr: middleware.ErrMissingAllowedMethods,
		},
		{
			name:    "negative max age",
			cfg:     func(cfg *middleware.CORSConfig) { cfg.MaxAge = -1 },
			wantErr: middleware.ErrInvalidMaxAge,
		},
		{
			name:    "exposed header with newline",
			cfg:     func(cfg *middleware.CORSConfig) { cfg.ExposedHeaders = []string{"X-A\r\nX-B: 1"} },
			wantErr: validate.ErrAllowedSymbols,
		},
		{
			name: "nil exposed headers",
			cfg:  func(cfg *middleware.CORSConfig) { cfg.ExposedHeaders = nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.CORSConfig{}
			cfg.SetDefaults()
			tt.cfg(cfg)

			err := cfg.Validate()
			if tt.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
