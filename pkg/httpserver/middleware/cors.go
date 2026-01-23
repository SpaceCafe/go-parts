package middleware

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/httpserver"
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

func (c *CORSConfig) Validate() error {
	if len(c.AllowedOrigins) == 0 {
		return ErrMissingAllowedOrigins
	}

	if len(c.AllowedMethods) == 0 {
		return ErrMissingAllowedMethods
	}

	if c.MaxAge < 0 {
		return ErrInvalidMaxAge
	}

	return nil
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

func containsWildcard(origins []string) bool {
	return slices.Contains(origins, "*")
}

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
