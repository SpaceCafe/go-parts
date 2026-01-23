package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg            func(*middleware.BasicAuthConfig)
		headers        map[string]string
		name           string
		username       string
		password       string
		wantAuthHeader string
		wantStatus     int
		basicAuth      bool
	}{
		{
			name: "valid token",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []string{"valid-token"}
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token valid-token",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid token",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []string{"valid-token"}
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token invalid-token",
			},
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Token",
		},
		{
			name:           "no auth provided",
			cfg:            func(_ *middleware.BasicAuthConfig) {},
			headers:        nil,
			basicAuth:      false,
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Basic realm=\"Restricted\"",
		},
		{
			name: "valid basic auth",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]string{"user": "pass"}
			},
			basicAuth:  true,
			username:   "user",
			password:   "pass",
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid basic auth",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]string{"user": "pass"}
			},
			basicAuth:      true,
			username:       "user",
			password:       "wrongpass",
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Basic realm=\"Restricted\"",
		},
		{
			name: "valid basic auth as token",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []string{"pass"}
				cfg.UseTokens = true
			},
			basicAuth:  true,
			username:   "user",
			password:   "pass",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.BasicAuthConfig{}
			cfg.SetDefaults()
			tt.cfg(cfg)

			handler := middleware.BasicAuth(
				cfg,
			)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.basicAuth {
				req.SetBasicAuth(tt.username, tt.password)
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "unexpected status code")

			if tt.wantAuthHeader != "" {
				assert.Contains(
					t,
					rec.Header().Get("WWW-Authenticate"),
					tt.wantAuthHeader,
					"unexpected 'WWW-Authenticate' header",
				)
			}
		})
	}
}
