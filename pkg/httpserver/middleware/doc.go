// Package middleware provides httpserver.Middleware implementations: BasicAuth (HTTP Basic and
// token authentication), CORS, Logger, MaxBodySize and RateLimit.
//
// Each middleware takes its own config or options; a nil config applies the defaults. Order
// matters when combining them: place RateLimit before BasicAuth, so failed logins cannot be used
// to exhaust the CPU, and MaxBodySize before handlers that read the body.
package middleware
