package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg                *middleware.CORSConfig
		expectedHeaders    map[string]string
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
			},
			requestOrigin:      "https://notallowed.com",
			requestMethod:      http.MethodOptions,
			expectedStatusCode: http.StatusNoContent,
			expectedHeaders:    map[string]string{}, // No CORS headers expected
		},
		{
			name: "credentials support enabled",
			cfg: &middleware.CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{http.MethodGet},
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
