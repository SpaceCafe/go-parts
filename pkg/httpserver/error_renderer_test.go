package httpserver_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errInvalidID  = errors.New("invalid id")
	errQuotedName = errors.New("field \"name\"\nmust not be empty")
)

func TestRenderErrorAsProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err        error
		name       string
		wantDetail string
	}{
		{name: "nil error uses status text", err: nil, wantDetail: "Bad Request"},
		{name: "error message", err: errInvalidID, wantDetail: "invalid id"},
		{
			name:       "message with quotes and newline",
			err:        errQuotedName,
			wantDetail: "field \"name\"\nmust not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

			httpserver.RenderErrorAsProblem(rec, req, http.StatusBadRequest, tt.err)

			var body struct {
				Type   string `json:"type"`
				Title  string `json:"title"`
				Detail string `json:"detail"`
				Status int    `json:"status"`
			}

			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(
				t,
				"application/problem+json; charset=utf-8",
				rec.Header().Get("Content-Type"),
			)
			assert.Equal(t, "/errors/bad-request", body.Type)
			assert.Equal(t, "Bad Request", body.Title)
			assert.Equal(t, http.StatusBadRequest, body.Status)
			assert.Equal(t, tt.wantDetail, body.Detail)
		})
	}
}
