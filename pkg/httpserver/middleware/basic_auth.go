package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/validate"
	"golang.org/x/crypto/bcrypt"
)

const (
	// authTokenPrefix is the Authorization header scheme used for token authentication, kept distinct
	// from the standard Basic scheme.
	authTokenPrefix = "Token "

	minSecretLength = 6
)

var (
	_ config.Defaultable = (*BasicAuthConfig)(nil)
	_ config.Validatable = (*BasicAuthConfig)(nil)

	ErrMismatchPassword = errors.New("basic-auth: password mismatch")

	//nolint:gochecknoglobals // Maintain a set of predefined bcrypt prefixes that are used throughout the application.
	BcryptHashPrefixes = []string{"$2a$", "$2b$", "$2x$", "$2y$"}
)

// Authenticator is a function type that validates a username and password, returning true if authentication succeeds.
type Authenticator func(username, password string) bool

// TokenAuthenticator validates a token from a "Token" Authorization header, returning true if
// authentication succeeds.
type TokenAuthenticator func(token string) bool

// BasicAuthConfig holds the configuration for BasicAuth middleware.
type BasicAuthConfig struct {
	// Principals defines a mapping of usernames to their respective passwords for basic
	// authentication. Passwords are secrets, so validation errors never echo them.
	Principals map[string]validate.Secret `json:"principals" yaml:"principals"`

	// Authenticator validates the username and password of HTTP Basic credentials.
	Authenticator Authenticator `env:"-" json:"-" yaml:"-"`

	// TokenAuthenticator validates a token when UseTokens is enabled. It is never consulted for
	// HTTP Basic credentials.
	TokenAuthenticator TokenAuthenticator `env:"-" json:"-" yaml:"-"`

	// Tokens defines a list of pre-approved tokens for token-based authentication. Tokens are
	// secrets, so validation errors never echo them.
	Tokens []validate.Secret `json:"tokens" yaml:"tokens"`

	// UseTokens indicates whether token-based authentication is enabled in addition to basic authentication.
	UseTokens bool `json:"useTokens" yaml:"useTokens"`
}

// SetDefaults initializes empty principal and token collections and installs the built-in
// authenticators that check credentials against them.
func (c *BasicAuthConfig) SetDefaults() {
	c.Principals = map[string]validate.Secret{}
	c.Tokens = []validate.Secret{}
	c.Authenticator = configAuthenticator(c)
	c.TokenAuthenticator = configTokenAuthenticator(c)
	c.UseTokens = false
}

// Validate ensures the credential collections and authenticator are non-nil, since a nil map or
// slice signals an unconfigured struct rather than a deliberately empty one.
func (c *BasicAuthConfig) Validate() error {
	return errors.Join(
		validate.Validate(
			"principals",
			c.Principals,
			validate.NotNilMap,
			validate.Entries[string](validate.LengthMin[validate.Secret](minSecretLength)),
		),
		validate.Validate(
			"tokens",
			c.Tokens,
			validate.NotNilSlice,
			validate.Elements(validate.LengthMin[validate.Secret](minSecretLength)),
		),
		validate.Validate("authenticator", &c.Authenticator, validate.NotNilPointer),
	)
}

// BasicAuth returns middleware that authenticates each request. When token auth is enabled, a valid
// token in a "Token" Authorization header is checked with TokenAuthenticator. HTTP Basic credentials
// are always checked with Authenticator, so tokens are never accepted as Basic passwords.
// Unauthenticated requests are aborted with a 401 and the challenges of every enabled scheme.
func BasicAuth(cfg *BasicAuthConfig) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if cfg.UseTokens && cfg.TokenAuthenticator != nil {
				authHeader := req.Header.Get("Authorization")

				if strings.HasPrefix(authHeader, authTokenPrefix) &&
					cfg.TokenAuthenticator(authHeader[len(authTokenPrefix):]) {
					next.ServeHTTP(resp, req)

					return
				}
			}

			username, password, ok := req.BasicAuth()
			if ok && cfg.Authenticator(username, password) {
				next.ServeHTTP(resp, req)

				return
			}

			abortBasicAuth(resp, req, cfg.UseTokens)
		})
	}
}

// configAuthenticator builds the default Authenticator over BasicAuthConfig. It looks the username
// up among the principals and compares its password.
func configAuthenticator(cfg *BasicAuthConfig) Authenticator {
	return func(username, password string) bool {
		if expectedPassword, ok := cfg.Principals[username]; ok {
			return ValidatePasswords(string(expectedPassword), password)
		}

		return false
	}
}

// configTokenAuthenticator builds the default TokenAuthenticator over BasicAuthConfig. It accepts a
// token that matches any configured token.
func configTokenAuthenticator(cfg *BasicAuthConfig) TokenAuthenticator {
	return func(token string) bool {
		for i := range cfg.Tokens {
			if ValidatePasswords(string(cfg.Tokens[i]), token) {
				return true
			}
		}

		return false
	}
}

// ValidatePasswords compares an expected password with an actual password,
// supporting bcrypt and byte-to-byte comparison.
func ValidatePasswords(expected, actual string) bool {
	validator := constantTimeCompare

	expectedBytes := []byte(expected)
	actualBytes := []byte(actual)

	for _, prefix := range BcryptHashPrefixes {
		if strings.HasPrefix(expected, prefix) {
			validator = bcrypt.CompareHashAndPassword
		}
	}

	return validator(expectedBytes, actualBytes) == nil
}

// abortBasicAuth writes a WWW-Authenticate challenge for every enabled scheme and aborts the
// request with a 401.
func abortBasicAuth(resp http.ResponseWriter, req *http.Request, useTokens bool) {
	resp.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

	if useTokens {
		resp.Header().Add("WWW-Authenticate", `Token`)
	}

	httpserver.Abort(resp, req, http.StatusUnauthorized, nil)
}

// constantTimeCompare compares two passwords for equality.
// Its behavior is undefined if the password length is > 2**31-1.
func constantTimeCompare(expected, actual []byte) error {
	if subtle.ConstantTimeCompare(expected, actual) == 1 {
		return nil
	}

	return ErrMismatchPassword
}
