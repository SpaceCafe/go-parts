package middleware_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
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
			name: "token scheme is case-insensitive",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "tOKEN valid-token",
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
				cfg.Principals = map[string]validate.Secret{"user": "password"}
			},
			basicAuth:  true,
			username:   "user",
			password:   "password",
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid basic auth",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Principals = map[string]validate.Secret{"user": "password"}
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
		{
			name: "empty token after scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token ",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "custom scheme in Authorization header",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenScheme = "Bearer"
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "bearer valid-token",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "default scheme rejected with custom scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenScheme = "Bearer"
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token valid-token",
			},
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Bearer",
		},
		{
			name: "custom header without scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenHeader = "X-API-Key"
				cfg.TokenScheme = ""
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"X-Api-Key": "valid-token",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "custom header without scheme rejects invalid token",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenHeader = "X-API-Key"
				cfg.TokenScheme = ""
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"X-API-Key": "Token valid-token",
			},
			wantStatus:     http.StatusUnauthorized,
			wantAuthHeader: "Basic realm=\"Restricted\"",
		},
		{
			name: "Authorization header ignored with custom header",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenHeader = "X-API-Key"
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"Authorization": "Token valid-token",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "custom header with scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.Tokens = []validate.Secret{"valid-token"}
				cfg.TokenHeader = "X-Auth"
				cfg.TokenScheme = "Key"
				cfg.UseTokens = true
			},
			headers: map[string]string{
				"X-Auth": "Key valid-token",
			},
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
		{
			name: "empty token header without tokens",
			cfg:  func(cfg *middleware.BasicAuthConfig) { cfg.TokenHeader = "" },
		},
		{
			name: "empty token header with tokens",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenHeader = ""
				cfg.UseTokens = true
			},
			wantErr: validate.ErrAllowedSymbols,
		},
		{
			name: "invalid token header",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenHeader = "X API Key"
				cfg.UseTokens = true
			},
			wantErr: validate.ErrAllowedSymbols,
		},
		{
			name: "invalid token scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenScheme = "Api Key"
				cfg.UseTokens = true
			},
			wantErr: validate.ErrAllowedSymbols,
		},
		{
			name: "empty token scheme",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenScheme = ""
				cfg.UseTokens = true
			},
		},
		{
			name: "basic token scheme in Authorization header",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenHeader = "authorization"
				cfg.TokenScheme = "basic"
				cfg.UseTokens = true
			},
			wantErr: middleware.ErrBasicTokenScheme,
		},
		{
			name: "basic token scheme in custom header",
			cfg: func(cfg *middleware.BasicAuthConfig) {
				cfg.TokenHeader = "X-Auth"
				cfg.TokenScheme = "Basic"
				cfg.UseTokens = true
			},
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

// Precomputed bcrypt hashes (cost 4) of "secret-pass" and "valid-token". Tests may not import
// bcrypt (depguard), so the fixtures are fixed strings.
const (
	bcryptSecretPass = "$2a$04$zUJ/Ut6PXVmFMdb3oBADjekubp4nzYc2WPX0gwrzXcQ8u6yRXoy2e" //nolint:gosec // Test fixture.
	bcryptValidToken = "$2a$04$..WxmjzhhI0w0N61ECRZWOuTkXq/yrev/mRXk6YT61gsKbEOSEr/e" //nolint:gosec // Test fixture.
)

func TestBasicAuth_SHA256Token(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("valid-token"))

	cfg := &middleware.BasicAuthConfig{}
	cfg.SetDefaults()
	cfg.Tokens = []validate.Secret{validate.Secret("sha256:" + hex.EncodeToString(digest[:]))}
	cfg.UseTokens = true
	require.NoError(t, cfg.Validate())

	handler := middleware.BasicAuth(
		cfg,
	)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for token, wantStatus := range map[string]int{
		"valid-token": http.StatusOK,
		"other-token": http.StatusUnauthorized,
		"sha256:" + hex.EncodeToString(digest[:]): http.StatusUnauthorized,
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Token "+token)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, wantStatus, rec.Code, "token %q", token)
	}
}

func TestBasicAuth_CustomTokenHeaderChallenge(t *testing.T) {
	t.Parallel()

	cfg := &middleware.BasicAuthConfig{}
	cfg.SetDefaults()
	cfg.Tokens = []validate.Secret{"valid-token"}
	cfg.TokenHeader = "X-API-Key"
	cfg.UseTokens = true

	handler := middleware.BasicAuth(
		cfg,
	)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(
		t,
		[]string{`Basic realm="Restricted"`},
		rec.Header().Values("WWW-Authenticate"),
		"a custom token header has no Authorization scheme to challenge",
	)
}

func TestBasicAuthConfig_Validate_TokenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		token   validate.Secret
	}{
		{name: "plaintext", token: "valid-token"},
		{name: "sha256 digest", token: "sha256:" + validate.Secret(strings.Repeat("ab", 32))},
		{
			name:    "bcrypt hash",
			token:   bcryptValidToken,
			wantErr: middleware.ErrBcryptToken,
		},
		{
			name:    "short sha256 digest",
			token:   "sha256:abcdef",
			wantErr: middleware.ErrInvalidTokenDigest,
		},
		{
			name:    "non-hex sha256 digest",
			token:   "sha256:" + validate.Secret(strings.Repeat("zz", 32)),
			wantErr: middleware.ErrInvalidTokenDigest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.BasicAuthConfig{}
			cfg.SetDefaults()
			cfg.Tokens = []validate.Secret{tt.token}

			err := cfg.Validate()
			if tt.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidatePasswords(t *testing.T) {
	t.Parallel()

	assert.True(t, middleware.ValidatePasswords("secret-pass", "secret-pass"))
	assert.False(t, middleware.ValidatePasswords("secret-pass", "secret"))
	assert.False(t, middleware.ValidatePasswords("secret-pass", "secret-pass-longer"))
	assert.True(t, middleware.ValidatePasswords(bcryptSecretPass, "secret-pass"))
	assert.False(t, middleware.ValidatePasswords(bcryptSecretPass, "wrong-pass"))
}

func TestNilConfigs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		middleware httpserver.Middleware
		wantStatus int
	}{
		"BasicAuth rejects everything": {middleware.BasicAuth(nil), http.StatusUnauthorized},
		"RateLimit applies defaults":   {middleware.RateLimit(t.Context(), nil), http.StatusOK},
		"CORS applies defaults":        {middleware.CORS(nil), http.StatusOK},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := tt.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(
				rec,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
			)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestInvalidConfigsPanic(t *testing.T) {
	t.Parallel()

	tests := map[string]func(){
		"BasicAuth without authenticator": func() {
			cfg := &middleware.BasicAuthConfig{}
			cfg.SetDefaults()
			cfg.Authenticator = nil
			middleware.BasicAuth(cfg)
		},
		"RateLimit without leak rate": func() {
			cfg := &middleware.RateLimitConfig{}
			cfg.SetDefaults()
			cfg.LeakRate = 0
			middleware.RateLimit(t.Context(), cfg)
		},
		"CORS with wildcard and credentials": func() {
			cfg := &middleware.CORSConfig{}
			cfg.SetDefaults()
			cfg.AllowedOrigins = []string{"*"}
			cfg.AllowCredentials = true
			middleware.CORS(cfg)
		},
	}

	for name, construct := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				err, ok := recover().(error)
				require.True(t, ok, "constructor must panic with an error")
				assert.ErrorIs(t, err, config.ErrValidation)
			}()

			construct()
		})
	}
}

func TestBasicAuthConfig_Validate_PasswordSchemes(t *testing.T) {
	t.Parallel()

	// The same password as bcryptSecretPass, hashed with cost 5 instead of 4.
	const bcryptCost5 = "$2a$05$I7tJsCFci7vNyp5sHPmPxuULrU0uKiIHSL5liL.UZ8v.kUGHD/iHm"

	tests := map[string]struct {
		principals map[string]validate.Secret
		wantErr    error
	}{
		"plaintext only": {
			principals: map[string]validate.Secret{"alice": "alicepass", "bob": "bobs-pass"},
		},
		"bcrypt with one cost": {
			principals: map[string]validate.Secret{
				"alice": bcryptSecretPass,
				"bob":   bcryptSecretPass,
			},
		},
		"bcrypt and plaintext": {
			principals: map[string]validate.Secret{"alice": bcryptSecretPass, "bob": "bobs-pass"},
			wantErr:    middleware.ErrMixedPasswordSchemes,
		},
		"bcrypt with different costs": {
			principals: map[string]validate.Secret{
				"alice": bcryptSecretPass,
				"bob":   bcryptCost5,
			},
			wantErr: middleware.ErrMixedPasswordSchemes,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := &middleware.BasicAuthConfig{}
			cfg.SetDefaults()
			cfg.Principals = tt.principals

			err := cfg.Validate()
			if tt.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), "bobs-pass")
		})
	}
}
