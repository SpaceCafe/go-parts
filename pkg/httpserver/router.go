package httpserver

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/spacecafe/go-parts/pkg/log"
)

var (
	_ Loggable        = (*Router)(nil)
	_ ErrorRenderable = (*Router)(nil)
)

// Middleware wraps a handler to run logic before or after it, composing into a chain.
type Middleware func(http.Handler) http.Handler

// ErrorRenderable is implemented by handlers that accept an ErrorRenderer, letting the HTTPServer
// propagate its renderer into the handler it serves.
type ErrorRenderable interface {
	SetErrorRenderer(renderer ErrorRenderer)
}

// Loggable is implemented by handlers that accept a logger, letting the HTTPServer propagate its
// logger into the handler it serves.
type Loggable interface {
	SetLogger(logger log.Logger)
}

// Router is an http.ServeMux augmented with two middleware chains. Global middleware runs on every
// request in ServeHTTP, while route middleware is baked into each handler at registration time so
// grouped routes can share middleware without affecting the rest of the mux.
type Router struct {
	*http.ServeMux

	Log log.Logger

	errorRenderer ErrorRenderer

	// globalChain wraps the whole mux and runs once per request, set on the top-level Router.
	globalChain []Middleware

	// routeChain is applied to individual handlers as they are registered, scoped to a Group.
	routeChain []Middleware

	// isSubRouter routes Use calls to routeChain instead of globalChain within a Group.
	isSubRouter bool
}

// NewRouter returns a Router backed by a fresh http.ServeMux and the default logger.
func NewRouter() *Router {
	return &Router{
		ServeMux: http.NewServeMux(),
		Log:      slog.Default(),
	}
}

// Group runs configure against a sub-router that shares the same mux but owns a cloned route chain.
// Middleware added inside the group therefore applies only to routes registered there, leaving the
// parent's chain untouched.
func (r *Router) Group(configure func(r *Router)) {
	subRouter := &Router{
		ServeMux:      r.ServeMux,
		Log:           r.Log,
		errorRenderer: r.errorRenderer,
		routeChain:    slices.Clone(r.routeChain),
		isSubRouter:   true,
	}
	configure(subRouter)
}

// Handle registers handler for pattern, wrapping it in the current route chain. The chain is applied
// in reverse, so the first middleware added is the outermost, matching intuitive ordering.
func (r *Router) Handle(pattern string, handler http.Handler) {
	for _, middleware := range slices.Backward(r.routeChain) {
		handler = middleware(handler)
	}

	r.ServeMux.Handle(pattern, handler)
}

// HandleFunc registers a handler function for pattern, applying the route chain like Handle.
func (r *Router) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Handle(pattern, handler)
}

// ServeHTTP wraps the response in a ResponseWriter, applies the global middleware chain, and
// dispatches to the mux. The chain is applied in reverse so the first middleware added runs first.
func (r *Router) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	var handler http.Handler = r.ServeMux

	writer := &ResponseWriter{ResponseWriter: resp, Log: r.Log, Error: r.errorRenderer}

	for _, middleware := range slices.Backward(r.globalChain) {
		handler = middleware(handler)
	}

	handler.ServeHTTP(writer, req)
}

// SetErrorRenderer sets the renderer used for error responses, satisfying ErrorRenderable.
func (r *Router) SetErrorRenderer(renderer ErrorRenderer) {
	r.errorRenderer = renderer
}

// SetLogger sets the logger, satisfying Loggable.
func (r *Router) SetLogger(logger log.Logger) {
	r.Log = logger
}

// Use appends middleware to the active chain. On a sub-router it extends the route chain, otherwise
// the global chain, so the same call behaves correctly inside and outside a Group.
func (r *Router) Use(middlewares ...Middleware) {
	if r.isSubRouter {
		r.routeChain = append(r.routeChain, middlewares...)
	} else {
		r.globalChain = append(r.globalChain, middlewares...)
	}
}
