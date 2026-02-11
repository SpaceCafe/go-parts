package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
	"golang.org/x/crypto/bcrypt"
)

const authTokenPrefix = "Token "

var (
	_ config.Defaultable = (*BasicAuthConfig)(nil)
	_ config.Validatable = (*BasicAuthConfig)(nil)

	ErrInvalidPrincipals    = errors.New("basic-auth: principals cannot be nil")
	ErrInvalidTokens        = errors.New("basic-auth: tokens cannot be nil")
	ErrInvalidAuthenticator = errors.New("basic-auth: authenticator cannot be nil")
	ErrMismatchPassword     = errors.New("basic-auth: password mismatch")

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

func (c *BasicAuthConfig) SetDefaults() {
	c.Principals = map[string]string{}
	c.Tokens = []string{}
	c.Authenticator = configAuthenticator(c)
	c.UseTokens = false
}

func (c *BasicAuthConfig) Validate() error {
	if c.Principals == nil {
		return ErrInvalidPrincipals
	}

	if c.Tokens == nil {
		return ErrInvalidTokens
	}

	if c.Authenticator == nil {
		return ErrInvalidAuthenticator
	}

	return nil
}

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

			abortBasicAuth(resp, cfg.UseTokens)
		})
	}
}

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

func abortBasicAuth(resp http.ResponseWriter, useTokens bool) {
	if useTokens {
		resp.Header().Set("WWW-Authenticate", `Token`)
	} else {
		resp.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	}

	http.Error(resp, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// constantTimeCompare compares two passwords for equality.
// Its behavior is undefined if the password length is > 2**31-1.
func constantTimeCompare(expected, actual []byte) error {
	if subtle.ConstantTimeCompare(expected, actual) == 1 {
		return nil
	}

	return ErrMismatchPassword
}
