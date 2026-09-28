package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/validate"
	"golang.org/x/crypto/bcrypt"
)

const (
	// authTokenScheme is the Authorization header scheme used for token authentication, kept distinct
	// from the standard Basic scheme. RFC 9110 defines schemes as case-insensitive.
	authTokenScheme = "Token"

	// sha256TokenPrefix marks a configured token stored as the hex SHA-256 digest of the token.
	sha256TokenPrefix = "sha256:"

	minSecretLength = 6
)

var (
	_ config.Defaultable = (*BasicAuthConfig)(nil)
	_ config.Validatable = (*BasicAuthConfig)(nil)

	ErrMismatchPassword = errors.New("basic-auth: password mismatch")
	ErrBcryptToken      = errors.New(
		"basic-auth: tokens must not be bcrypt hashes, use plaintext or sha256:<hex>",
	)
	ErrInvalidTokenDigest   = errors.New("basic-auth: sha256 token must be 64 hex characters")
	ErrMixedPasswordSchemes = errors.New(
		"basic-auth: principals must all be plaintext or all bcrypt with the same cost",
	)

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
	// authentication. Passwords are secrets, so validation errors never echo them. All passwords
	// must be plaintext, or all bcrypt hashes with the same cost, so response times cannot reveal
	// which usernames exist.
	Principals map[string]validate.Secret `json:"principals" yaml:"principals"`

	// Authenticator validates the username and password of HTTP Basic credentials.
	Authenticator Authenticator `env:"-" json:"-" yaml:"-"`

	// TokenAuthenticator validates a token when UseTokens is enabled. It is never consulted for
	// HTTP Basic credentials.
	TokenAuthenticator TokenAuthenticator `env:"-" json:"-" yaml:"-"`

	// Tokens defines a list of pre-approved tokens for token-based authentication. Each entry is
	// either the plaintext token or "sha256:" followed by the hex SHA-256 digest of the token.
	// bcrypt hashes are rejected: tokens are high-entropy, so bcrypt adds no protection, but it
	// would cost one slow comparison per configured token on every request. Tokens are secrets, so
	// validation errors never echo them.
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

// Validate ensures the credential collections and Authenticator are non-nil, since a nil map or
// slice signals an unconfigured struct rather than a deliberately empty one. TokenAuthenticator
// must be non-nil when UseTokens is enabled. Both authenticators are excluded from config files, so
// a struct decoded without SetDefaults fails here instead of panicking on the first request.
func (c *BasicAuthConfig) Validate() error {
	return errors.Join(
		validate.Validate(
			"principals",
			c.Principals,
			validate.NotNilMap,
			validate.Entries[string](validate.RuneLengthMin[validate.Secret](minSecretLength)),
			validatePasswordSchemes,
		),
		validate.Validate(
			"tokens",
			c.Tokens,
			validate.NotNilSlice,
			validate.Elements(
				validate.RuneLengthMin[validate.Secret](minSecretLength),
				validateTokenFormat,
			),
		),
		validate.Validate("authenticator", c.Authenticator, func(value Authenticator) error {
			if value == nil {
				return validate.ErrNil
			}

			return nil
		}),
		validate.Validate(
			"token authenticator",
			c.TokenAuthenticator,
			func(value TokenAuthenticator) error {
				if c.UseTokens && value == nil {
					return validate.ErrNil
				}

				return nil
			},
		),
	)
}

// BasicAuth returns middleware that authenticates each request. When token auth is enabled, a valid
// token in a "Token" Authorization header is checked with TokenAuthenticator. HTTP Basic credentials
// are always checked with Authenticator, so tokens are never accepted as Basic passwords.
// Unauthenticated requests are aborted with a 401 and the challenges of every enabled scheme.
// Place RateLimit before BasicAuth: bcrypt principals make every failed login deliberately slow, so
// unthrottled clients can use them to exhaust the CPU.
//
// A nil cfg applies the defaults: no principals and no tokens, so every request is rejected. An
// invalid cfg panics (see config.MustValidate).
func BasicAuth(cfg *BasicAuthConfig) httpserver.Middleware {
	if cfg == nil {
		cfg = &BasicAuthConfig{}
		cfg.SetDefaults()
	}

	config.MustValidate(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if cfg.UseTokens && cfg.TokenAuthenticator != nil {
				scheme, token, found := strings.Cut(req.Header.Get("Authorization"), " ")

				if found && strings.EqualFold(scheme, authTokenScheme) &&
					cfg.TokenAuthenticator(token) {
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
// up among the principals and compares its password. An unknown username still runs a comparison
// against a configured password, so the response time does not reveal which usernames exist.
func configAuthenticator(cfg *BasicAuthConfig) Authenticator {
	return func(username, password string) bool {
		expectedPassword, ok := cfg.Principals[username]
		if !ok {
			_ = ValidatePasswords(dummyPassword(cfg.Principals), password)

			return false
		}

		return ValidatePasswords(string(expectedPassword), password)
	}
}

// dummyPassword returns a configured bcrypt hash, so a lookup for an unknown username costs the
// same as for a known one. Without bcrypt principals, any plaintext comparison has the same cost,
// so an empty string is enough.
func dummyPassword(principals map[string]validate.Secret) string {
	for _, password := range principals {
		if isBcryptHash(string(password)) {
			return string(password)
		}
	}

	return ""
}

// configTokenAuthenticator builds the default TokenAuthenticator over BasicAuthConfig. It hashes the
// presented token once and compares the fixed-length digest with every configured token without an
// early exit, so neither the token length nor the matching position shows in the response time.
func configTokenAuthenticator(cfg *BasicAuthConfig) TokenAuthenticator {
	return func(token string) bool {
		presented := sha256.Sum256([]byte(token))
		match := 0

		for _, stored := range cfg.Tokens {
			digest, err := tokenDigest(string(stored))
			if err != nil {
				continue
			}

			match |= subtle.ConstantTimeCompare(digest, presented[:])
		}

		return match == 1
	}
}

// tokenDigest returns the SHA-256 digest of a configured token, decoding it for a "sha256:" entry.
func tokenDigest(stored string) ([]byte, error) {
	if hexDigest, ok := strings.CutPrefix(stored, sha256TokenPrefix); ok {
		digest, err := hex.DecodeString(hexDigest)
		if err != nil || len(digest) != sha256.Size {
			return nil, ErrInvalidTokenDigest
		}

		return digest, nil
	}

	digest := sha256.Sum256([]byte(stored))

	return digest[:], nil
}

// validateTokenFormat rejects bcrypt tokens and malformed "sha256:" digests.
func validateTokenFormat(token validate.Secret) error {
	if isBcryptHash(string(token)) {
		return ErrBcryptToken
	}

	_, err := tokenDigest(string(token))

	return err
}

// validatePasswordSchemes rejects principals that mix plaintext and bcrypt passwords or bcrypt
// costs. An unknown username is checked against a bcrypt dummy whenever any principal uses bcrypt
// (see dummyPassword), so a known user with a cheaper scheme answers measurably faster and reveals
// that the username exists.
func validatePasswordSchemes(principals map[string]validate.Secret) error {
	schemes := make(map[string]struct{}, 1)

	for _, password := range principals {
		scheme := "plaintext"

		if isBcryptHash(string(password)) {
			cost, err := bcrypt.Cost([]byte(password))
			if err != nil {
				return fmt.Errorf("invalid bcrypt hash: %w", err)
			}

			scheme = "bcrypt cost " + strconv.Itoa(cost)
		}

		schemes[scheme] = struct{}{}
	}

	if len(schemes) > 1 {
		return fmt.Errorf(
			"%w: found %s",
			ErrMixedPasswordSchemes,
			strings.Join(slices.Sorted(maps.Keys(schemes)), ", "),
		)
	}

	return nil
}

// isBcryptHash reports whether value starts with one of the BcryptHashPrefixes.
func isBcryptHash(value string) bool {
	for _, prefix := range BcryptHashPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
}

// ValidatePasswords compares an expected password with an actual password, supporting bcrypt
// hashes and plaintext. Plaintext passwords are compared as SHA-256 digests, so the comparison
// takes the same time whatever the lengths of the two passwords.
func ValidatePasswords(expected, actual string) bool {
	if isBcryptHash(expected) {
		return bcrypt.CompareHashAndPassword([]byte(expected), []byte(actual)) == nil
	}

	expectedDigest := sha256.Sum256([]byte(expected))
	actualDigest := sha256.Sum256([]byte(actual))

	return constantTimeCompare(expectedDigest[:], actualDigest[:]) == nil
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
