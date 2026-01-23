package httpserver

import (
	"net/http"
	"slices"
)

type Middleware func(http.Handler) http.Handler

type Router struct {
	*http.ServeMux

	globalChain []Middleware
	routeChain  []Middleware
	isSubRouter bool
}

func NewRouter() *Router {
	return &Router{ServeMux: http.NewServeMux()}
}

func (r *Router) Group(configure func(r *Router)) {
	subRouter := &Router{
		routeChain:  slices.Clone(r.routeChain),
		isSubRouter: true,
		ServeMux:    r.ServeMux,
	}
	configure(subRouter)
}

func (r *Router) Handle(pattern string, handler http.Handler) {
	for _, middleware := range slices.Backward(r.routeChain) {
		handler = middleware(handler)
	}

	r.ServeMux.Handle(pattern, handler)
}

func (r *Router) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Handle(pattern, handler)
}

func (r *Router) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	var handler http.Handler = r.ServeMux

	for _, middleware := range slices.Backward(r.globalChain) {
		handler = middleware(handler)
	}

	handler.ServeHTTP(resp, req)
}

func (r *Router) Use(middlewares ...Middleware) {
	if r.isSubRouter {
		r.routeChain = append(r.routeChain, middlewares...)
	} else {
		r.globalChain = append(r.globalChain, middlewares...)
	}
}
