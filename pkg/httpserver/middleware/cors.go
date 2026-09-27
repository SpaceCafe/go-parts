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
		validate.Validate(
			"allowed origins",
			c.AllowedOrigins,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty),
		),
		validate.Validate(
			"allowed methods",
			c.AllowedMethods,
			validate.NotNilSlice,
			validate.Elements[string](validate.AllowedValues(httpMethods)),
		),
		validate.Validate(
			"allowed headers",
			c.AllowedHeaders,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty, validate.PrintableASCII),
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

			setVaryHeaders(resp, req, allowAllOrigins)

			setCORSHeaders(resp, allowOrigin, cfg.AllowCredentials, exposeHeaders)

			if isPreflight(req) {
				handlePreflightRequest(
					resp,
					req,
					allowOrigin,
					allowMethods,
					allowHeaders,
					maxAge,
				)

				return
			}

			next.ServeHTTP(resp, req)
		})
	}
}

// setVaryHeaders tells caches which request headers the CORS response depends on. Unless every
// origin is allowed, the response differs per Origin, so a shared cache must not serve one origin's
// response to another. An OPTIONS response also depends on whether the request is a preflight.
// Vary is added even when no CORS headers are written, because that absence is origin-specific too.
func setVaryHeaders(resp http.ResponseWriter, req *http.Request, allowAllOrigins bool) {
	if !allowAllOrigins {
		resp.Header().Add("Vary", "Origin")
	}

	if req.Method == http.MethodOptions {
		resp.Header().Add("Vary", "Access-Control-Request-Method")
		resp.Header().Add("Vary", "Access-Control-Request-Headers")
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

// isPreflight reports whether req is a CORS preflight. A plain OPTIONS request, without Origin or
// Access-Control-Request-Method, is passed on to the handler like any other request.
func isPreflight(req *http.Request) bool {
	return req.Method == http.MethodOptions &&
		req.Header.Get("Origin") != "" &&
		req.Header.Get("Access-Control-Request-Method") != ""
}

// handlePreflightRequest answers a preflight. An allowed origin gets the allowed methods, headers,
// and cache duration with "204 No Content". A disallowed origin gets "403 Forbidden", so the
// rejection is visible in the network log instead of looking like a successful preflight.
func handlePreflightRequest(
	resp http.ResponseWriter,
	req *http.Request,
	allowOrigin, allowMethods, allowHeaders, maxAge string,
) {
	if allowOrigin == "" {
		httpserver.Abort(resp, req, http.StatusForbidden, nil)

		return
	}

	resp.Header().Set("Access-Control-Allow-Methods", allowMethods)

	if allowHeaders != "" {
		resp.Header().Set("Access-Control-Allow-Headers", allowHeaders)
	}

	if maxAge != "" {
		resp.Header().Set("Access-Control-Max-Age", maxAge)
	}

	resp.WriteHeader(http.StatusNoContent)
}
