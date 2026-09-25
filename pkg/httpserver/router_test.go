package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
)

func TestRouter_AbortWithoutHTTPServer(t *testing.T) {
	t.Parallel()

	router := httpserver.NewRouter()
	router.SetLogger(&mockLogger{})
	router.HandleFunc("GET /", func(resp http.ResponseWriter, req *http.Request) {
		httpserver.Abort(resp, req, http.StatusUnauthorized, nil)
	})

	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		router.ServeHTTP(
			rec,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
		)
	})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "Unauthorized\n", rec.Body.String())
}

func TestResponseWriter_Abort_ZeroValue(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	resp := &httpserver.ResponseWriter{ResponseWriter: rec}

	assert.NotPanics(t, func() {
		resp.Abort(
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
			http.StatusTooManyRequests,
			nil,
		)
	})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
