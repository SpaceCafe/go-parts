package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				cfg.Tokens = []validate.Secret{"valid-token"}
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
				cfg.Tokens = []validate.Secret{"valid-token"}
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
				cfg.Principals = map[string]validate.Secret{"user": "pass"}
			},
			basicAuth:  true,
			username:   "user",
			password:   "pass",
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid basic auth",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]validate.Secret{"user": "pass"}
			},
			basicAuth:      true,
			username:       "user",
			password:       "wrongpass",
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Basic realm=\"Restricted\"",
		},
		{
			name: "token rejected as basic password",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.UseTokens = true
			},
			basicAuth:      true,
			username:       "anyone",
			password:       "valid-token",
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Basic realm=\"Restricted\"",
		},
		{
			name: "principal login with tokens enabled",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]validate.Secret{"alice": "alicepass"}
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.UseTokens = true
			},
			basicAuth:  true,
			username:   "alice",
			password:   "alicepass",
			wantStatus: http.StatusOK,
		},
		{
			name: "principal password rejected as token",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]validate.Secret{"alice": "alicepass"}
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token alicepass",
			},
			wantStatus: http.StatusUnauthorized,
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

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
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
					rec.Header().Values("WWW-Authenticate"),
					tt.wantAuthHeader,
					"unexpected 'WWW-Authenticate' header",
				)
			}
		})
	}
}

func TestBasicAuthConfig_Validate_RedactsSecrets(t *testing.T) {
	t.Parallel()

	cfg := &middleware.BasicAuthConfig{}
	cfg.SetDefaults()
	cfg.Principals = map[string]validate.Secret{"alice": "alicepass", "bob": "short"}
	cfg.Tokens = []validate.Secret{"qz9"}

	err := cfg.Validate()
	require.ErrorIs(t, err, validate.ErrLengthMin)

	for _, secret := range []string{"alicepass", "short", "qz9"} {
		assert.NotContains(t, err.Error(), secret)
	}
}

func TestBasicAuthConfig_Validate_Authenticators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg     func(*middleware.BasicAuthConfig)
		wantErr error
		name    string
	}{
		{name: "defaults", cfg: func(*middleware.BasicAuthConfig) {}},
		{
			name:    "nil authenticator",
			cfg:     func(cfg *middleware.BasicAuthConfig) { cfg.Authenticator = nil },
			wantErr: validate.ErrNil,
		},
		{
			name: "nil token authenticator without tokens",
			cfg:  func(cfg *middleware.BasicAuthConfig) { cfg.TokenAuthenticator = nil },
		},
		{
			name: "nil token authenticator with tokens",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenAuthenticator = nil
				cfg.UseTokens = true
			},
			wantErr: validate.ErrNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.BasicAuthConfig{}
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
