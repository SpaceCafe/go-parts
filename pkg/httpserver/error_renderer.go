package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

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

	_, _ = resp.Write([]byte(`{"type": "/errors/`))
	_, _ = resp.Write([]byte(typeconv.ToKebabCase(statusText)))
	_, _ = resp.Write([]byte(`", "title": "`))
	_, _ = resp.Write([]byte(statusText))
	_, _ = resp.Write([]byte(`", "status": `))
	_, _ = resp.Write([]byte(strconv.Itoa(code)))
	_, _ = resp.Write([]byte(`, "detail": "`))

	if err == nil || err.Error() == "" {
		_, _ = resp.Write([]byte(statusText))
	} else {
		//nolint:errchkjson // Error encoding is intentionally ignored as this is already an error handler.
		_ = json.NewEncoder(resp).Encode(err.Error())
	}

	_, _ = resp.Write([]byte(`"}`))
}
