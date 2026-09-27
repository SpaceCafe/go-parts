// Package middleware provides httpserver.Middleware implementations: BasicAuth (HTTP Basic and
// token authentication), CORS, Logger, MaxBodySize, RateLimit and Recover.
//
// Each middleware takes its own config or options; a nil config applies the defaults.
//
// We strongly suggest adding them to the top-level httpserver.Router with Use, in this order:
//
//	router.Use(
//		middleware.Recover(logger),
//		middleware.Logger(logger),
//		middleware.RateLimit(ctx, &cfg.RateLimit),
//		middleware.MaxBodySize(10 << 20),
//		middleware.CORS(&cfg.CORS),
//		middleware.BasicAuth(&cfg.BasicAuth),
//	)
//
// The order matters:
//   - Recover comes first, so it catches panics in every later middleware and handler.
//   - Logger comes next, so it logs requests that later middlewares reject.
//   - RateLimit precedes BasicAuth, so failed logins cannot be used to exhaust the CPU.
//   - MaxBodySize precedes every handler that reads the body. A route can raise the limit with its
//     own MaxBodySize, for example for uploads.
//   - CORS precedes BasicAuth, so preflight requests, which carry no credentials, get answered.
//
// Leave out the middlewares you do not need, but keep the order of the rest. CORS and BasicAuth
// need a deliberate configuration: the CORS defaults allow any origin, and the BasicAuth defaults
// reject every request.
package middleware
