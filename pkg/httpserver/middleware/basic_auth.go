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

// BasicAuthConfig holds the configuration for BasicAuth middleware.
type BasicAuthConfig struct {
	// Principals defines a mapping of usernames to their respective passwords for basic authentication.
	Principals map[string]string `json:"principals" yaml:"principals"`

	// Authenticator validates a username and password.
	Authenticator Authenticator

	// Tokens defines a list of pre-approved tokens for token-based authentication.
	Tokens []string `json:"tokens" yaml:"tokens"`

	// UseTokens indicates whether token-based authentication is enabled in addition to basic authentication.
	UseTokens bool
}

// SetDefaults initializes empty principal and token collections and installs the built-in
// authenticator that checks credentials against them.
func (c *BasicAuthConfig) SetDefaults() {
	c.Principals = map[string]string{}
	c.Tokens = []string{}
	c.Authenticator = configAuthenticator(c)
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
			validate.Entries[string, string](validate.LengthMin[string](minSecretLength)),
		),
		validate.Validate(
			"tokens",
			c.Tokens,
			validate.NotNilSlice,
			validate.Elements[string](validate.LengthMin[string](minSecretLength)),
		),
		validate.Validate("authenticator", &c.Authenticator, validate.NotNilPointer),
	)
}

// BasicAuth returns middleware that authenticates each request. When token auth is enabled, a valid
// bearer token in the Authorization header is accepted first, otherwise HTTP Basic credentials are
// checked. Unauthenticated requests are aborted with a 401 and the appropriate challenge.
func BasicAuth(cfg *BasicAuthConfig) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if cfg.UseTokens {
				authHeader := req.Header.Get("Authorization")

				if strings.HasPrefix(authHeader, authTokenPrefix) &&
					cfg.Authenticator("", authHeader[len(authTokenPrefix):]) {
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

// configAuthenticator builds the default Authenticator over BasicAuthConfig. In token mode it accepts any
// password matching a configured token and ignores the username, otherwise it looks the username up
// among the principals and compares its password.
func configAuthenticator(cfg *BasicAuthConfig) Authenticator {
	return func(username, password string) bool {
		if cfg.UseTokens {
			for i := range cfg.Tokens {
				ok := ValidatePasswords(cfg.Tokens[i], password)
				if ok {
					return true
				}
			}

			return false
		}

		if expectedPassword, ok := cfg.Principals[username]; ok {
			return ValidatePasswords(expectedPassword, password)
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

// abortBasicAuth writes the WWW-Authenticate challenge matching the configured scheme and aborts
// the request with a 401.
func abortBasicAuth(resp http.ResponseWriter, req *http.Request, useTokens bool) {
	if useTokens {
		resp.Header().Set("WWW-Authenticate", `Token`)
	} else {
		resp.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
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
