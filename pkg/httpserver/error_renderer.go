package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/spacecafe/go-parts/pkg/typeconv"
)

// ErrorRenderer writes an error response. The detail passed in has already been reduced to what the
// client is allowed to see.
type ErrorRenderer func(resp http.ResponseWriter, req *http.Request, code int, err error)

// RenderErrorAsText sends an HTTP error response with the specified status code and error message.
func RenderErrorAsText(resp http.ResponseWriter, _ *http.Request, code int, err error) {
	if err == nil || err.Error() == "" {
		http.Error(resp, http.StatusText(code), code)

		return
	}

	http.Error(resp, err.Error(), code)
}

// RenderErrorAsProblem constructs and sends a problem+json compliant error response (RFC 7807)
// with the given status code and error details.
func RenderErrorAsProblem(resp http.ResponseWriter, _ *http.Request, code int, err error) {
	header := resp.Header()
	header.Del("Content-Length")
	header.Set("Content-Type", "application/problem+json; charset=utf-8")
	header.Set("X-Content-Type-Options", "nosniff")
	resp.WriteHeader(code)

	statusText := http.StatusText(code)

	detail := statusText
	if err != nil && err.Error() != "" {
		detail = err.Error()
	}

	// Encode the whole document at once, so the detail is escaped as a proper JSON string.
	//nolint:errchkjson // Error encoding is intentionally ignored as this is already an error handler.
	_ = json.NewEncoder(resp).Encode(problem{
		Type:   "/errors/" + typeconv.ToKebabCase(statusText),
		Title:  statusText,
		Status: code,
		Detail: detail,
	})
}

// problem is the RFC 7807 problem details document written by RenderErrorAsProblem.
type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}
