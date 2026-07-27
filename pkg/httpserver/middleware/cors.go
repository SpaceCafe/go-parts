package middleware

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/validate"
)

var (
	_ config.Defaultable = (*CORSConfig)(nil)
	_ config.Validatable = (*CORSConfig)(nil)

	ErrMissingAllowedOrigins = errors.New("CORS: allowed origins cannot be empty")
	ErrMissingAllowedMethods = errors.New("CORS: allowed methods cannot be empty")
	ErrInvalidMaxAge         = errors.New("CORS: max age must be non-negative")
)

// CORSConfig holds the configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present, all origins will be allowed.
	// Default: ["*"]
	AllowedOrigins []string `json:"allowedOrigins" yaml:"allowedOrigins"`

	// AllowedMethods is a list of methods the client is allowed to use with cross-domain requests.
	// Default: ["HEAD", "GET", "POST"]
	AllowedMethods []string `json:"allowedMethods" yaml:"allowedMethods"`

	// AllowedHeaders is a list of headers the client is allowed to use with cross-domain requests.
	// Default: ["Accept", "Authorization", "Content-Type", "X-CSRF-Token"]
	AllowedHeaders []string `json:"allowedHeaders" yaml:"allowedHeaders"`

	// ExposedHeaders indicates which headers are safe to expose to the API of a CORS response.
	// Default: []
	ExposedHeaders []string `json:"exposedHeaders" yaml:"exposedHeaders"`

	// MaxAge indicates how long (in seconds) the results of a preflight request can be cached.
	// Default: 0 (no cache)
	MaxAge int `json:"maxAge" yaml:"maxAge"`

	// AllowCredentials indicates whether the request can include user credentials.
	// Default: false
	AllowCredentials bool `json:"allowCredentials" yaml:"allowCredentials"`
}

// SetDefaults applies a permissive but safe baseline, allowing any origin with the common safe
// methods and headers, no credentials, and no preflight caching.
func (c *CORSConfig) SetDefaults() {
	c.AllowedOrigins = []string{"*"}
	c.AllowedMethods = []string{
		http.MethodHead,
		http.MethodGet,
		http.MethodPost,
	}
	c.AllowedHeaders = []string{
		"Accept",
		"Authorization",
		"Content-Type",
		"X-CSRF-Token",
	}
	c.ExposedHeaders = []string{}
	c.MaxAge = 0
	c.AllowCredentials = false
}

// Validate ensures origins and methods are present and that every configured method is a real HTTP
// method and every header is printable ASCII, rejecting values that would produce malformed headers.
func (c *CORSConfig) Validate() error {
	httpMethods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}

	return errors.Join(
		validate.Validate("allowed origins", c.AllowedOrigins, validate.NotEmptySlice),
		validate.ValidateSlice("allowed origins", c.AllowedOrigins, validate.NotEmpty),
		validate.Validate("allowed methods", c.AllowedMethods, validate.NotEmptySlice),
		validate.ValidateSlice(
			"allowed methods",
			c.AllowedMethods,
			validate.NotEmpty,
			validate.AllowedValues(httpMethods),
		),
		validate.ValidateSlice(
			"allowed headers",
			c.AllowedHeaders,
			validate.NotEmpty,
			validate.PrintableASCII,
		),
		validate.Validate("max age", c.MaxAge, validate.NonNegative),
	)
}

// CORS returns a middleware that enables Cross-Origin Resource Sharing (CORS).
func CORS(cfg *CORSConfig) httpserver.Middleware {
	if cfg == nil {
		cfg = &CORSConfig{}
		cfg.SetDefaults()
	}

	allowAllOrigins := containsWildcard(cfg.AllowedOrigins)

	// Pre-build header values.
	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposedHeaders, ", ")

	maxAge := ""
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(cfg.MaxAge)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			allowOrigin := getAllowedOrigin(origin, cfg.AllowedOrigins, allowAllOrigins)

			setCORSHeaders(resp, allowOrigin, cfg.AllowCredentials, exposeHeaders)

			if req.Method == http.MethodOptions {
				handlePreflightRequest(resp, allowOrigin, allowMethods, allowHeaders, maxAge)

				return
			}

			next.ServeHTTP(resp, req)
		})
	}
}

// containsWildcard reports whether origins contains the "*" wildcard that allows any origin.
func containsWildcard(origins []string) bool {
	return slices.Contains(origins, "*")
}

// getAllowedOrigin resolves the value for the Access-Control-Allow-Origin header. It returns "*"
// when all origins are allowed, the request origin when it is explicitly listed, and an empty string
// otherwise so no header is emitted for a disallowed origin.
func getAllowedOrigin(origin string, allowedOrigins []string, allowAll bool) string {
	if allowAll {
		return "*"
	}

	if origin == "" {
		return ""
	}

	if slices.Contains(allowedOrigins, origin) {
		return origin
	}

	return ""
}

// setCORSHeaders writes the response headers common to simple and preflight requests. It is a no-op
// for a disallowed (empty) origin so cross-origin responses are not exposed.
func setCORSHeaders(
	resp http.ResponseWriter,
	allowOrigin string,
	allowCredentials bool,
	exposeHeaders string,
) {
	if allowOrigin == "" {
		return
	}

	resp.Header().Set("Access-Control-Allow-Origin", allowOrigin)

	if allowCredentials {
		resp.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if exposeHeaders != "" {
		resp.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
	}
}

// handlePreflightRequest answers an OPTIONS preflight, adding the allowed methods, headers, and
// cache duration for an allowed origin, and always responds with "204 No Content".
func handlePreflightRequest(
	resp http.ResponseWriter,
	allowOrigin, allowMethods, allowHeaders, maxAge string,
) {
	if allowOrigin != "" {
		resp.Header().Set("Access-Control-Allow-Methods", allowMethods)

		if allowHeaders != "" {
			resp.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}

		if maxAge != "" {
			resp.Header().Set("Access-Control-Max-Age", maxAge)
		}
	}

	resp.WriteHeader(http.StatusNoContent)
}
