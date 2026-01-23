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

	ErrMismatchPassword = errors.New("basic-auth: password mismatch")

	//nolint:gochecknoglobals // Maintain a set of predefined bcrypt prefixes that are used throughout the application.
	BcryptHashPrefixes = []string{"$2a$", "$2b$", "$2x$", "$2y$"}
)

type Authenticator func(username, password string) bool

type BasicAuthConfig struct {
	Principals    map[string]string `json:"principals" yaml:"principals"`
	Authenticator Authenticator
	Tokens        []string `json:"tokens"     yaml:"tokens"`
	UseTokens     bool
}

func (c *BasicAuthConfig) SetDefaults() {
	c.Principals = map[string]string{}
	c.Tokens = []string{}
	c.Authenticator = configAuthenticator(c)
	c.UseTokens = false
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

	http.Error(resp, "Unauthorized", http.StatusUnauthorized)
}

// constantTimeCompare compares two passwords for equality.
// Its behavior is undefined if the password length is > 2**31-1.
func constantTimeCompare(expected, actual []byte) error {
	if subtle.ConstantTimeCompare(expected, actual) == 1 {
		return nil
	}

	return ErrMismatchPassword
}
