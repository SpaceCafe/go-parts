package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/stretchr/testify/assert"
)

// panicHandler returns a handler that panics with value.
func panicHandler(value any) http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(value)
	})
}

func TestRecover(t *testing.T) {
	t.Parallel()

	handler := middleware.Recover(&queryLogger{})(panicHandler("boom"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
	)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom", "the panic value must not leak to the client")
}

func TestRecover_ErrAbortHandler(t *testing.T) {
	t.Parallel()

	handler := middleware.Recover(&queryLogger{})(panicHandler(http.ErrAbortHandler))

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		handler.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
		)
	})
}

func TestRecover_AfterWrite(t *testing.T) {
	t.Parallel()

	handler := middleware.Recover(&queryLogger{})(
		http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			_, _ = resp.Write([]byte("partial"))

			panic("boom")
		}),
	)

	rec := httptest.NewRecorder()

	// The Router wraps every response in a ResponseWriter, which records that the response started.
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		handler.ServeHTTP(
			&httpserver.ResponseWriter{ResponseWriter: rec},
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody),
		)
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "partial", rec.Body.String(), "no error body may follow the partial response")
}
