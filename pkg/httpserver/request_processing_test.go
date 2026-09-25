package httpserver_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInvalidPayload = errors.New("invalid payload")

type uploadPayload struct {
	Name  string             `json:"name"`
	Files []*httpserver.File `json:"files"`
}

type validatedPayload struct {
	File httpserver.Base64File `json:"file"`
}

func (p *validatedPayload) Validate() error { return errInvalidPayload }

func TestGetJSONBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		body    string
	}{
		{name: "valid body", body: `{"name":"a"}`},
		{name: "trailing whitespace", body: "{\"name\":\"a\"}\n\t "},
		{name: "unknown field", body: `{"title":"a"}`, wantErr: httpserver.ErrJSONBodyDecoding},
		{
			name:    "trailing garbage",
			body:    `{"name":"a"}garbage`,
			wantErr: httpserver.ErrJSONBodyDecoding,
		},
		{name: "second value", body: `{"name":"a"}{}`, wantErr: httpserver.ErrJSONBodyDecoding},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var payload uploadPayload

			err := httpserver.GetJSONBody(newBodyRequest(t, strings.NewReader(tt.body)), &payload)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "a", payload.Name)
		})
	}
}

func TestGetJSONBody_CleansUpFilesOnError(t *testing.T) {
	t.Parallel()

	t.Run("decode error after file field", func(t *testing.T) {
		t.Parallel()

		var payload uploadPayload

		body := `{"files":["one","two"],"name":"a"}garbage`
		err := httpserver.GetJSONBody(newBodyRequest(t, strings.NewReader(body)), &payload)
		require.ErrorIs(t, err, httpserver.ErrJSONBodyDecoding)
		require.Len(t, payload.Files, 2)

		for _, file := range payload.Files {
			assert.NoDirExists(t, file.Dir)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()

		var payload validatedPayload

		body := `{"file":"cGF5bG9hZA=="}`
		err := httpserver.GetJSONBody(newBodyRequest(t, strings.NewReader(body)), &payload)
		require.ErrorIs(t, err, errInvalidPayload)
		require.NotEmpty(t, payload.File.Dir)

		_, statErr := os.Stat(payload.File.Dir)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})
}
