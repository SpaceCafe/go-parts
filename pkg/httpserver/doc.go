// Package httpserver wraps net/http with a configurable server, a router with middleware chains,
// and helpers for reading requests and writing error responses.
//
// HTTPServer starts and stops an http.Server from Config (TLS, timeouts, HTTP/2 and H2C) and fits
// the shutdown package's Trackable interface. Router is an http.ServeMux with global middleware
// and per-group route middleware. Handlers end failed requests with Abort, which logs the error
// and renders it with the configured ErrorRenderer (plain text or RFC 7807 problem details);
// server errors and RedactedError values never reveal their detail to the client.
//
// GetJSONBody, GetFileFromBody, GetFormValue, GetPathValue and GetQueryParam read and validate
// request input. Request bodies are not limited here; see middleware.MaxBodySize.
package httpserver
