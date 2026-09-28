package httpserver_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/httpserver/middleware"
	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFormLimit = 1024

func newFormRequest(t *testing.T, target string, body io.Reader, contentType string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, body)
	req.Header.Set("Content-Type", contentType)

	return req
}

func urlEncodedBody(value string) (body io.Reader, contentType string) {
	body = strings.NewReader(url.Values{"key": {value}}.Encode())

	return body, "application/x-www-form-urlencoded"
}

// multipartBody writes into a bytes.Buffer, which cannot fail, so errors are ignored.
func multipartBody(value string) (body io.Reader, contentType string) {
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("key", value)
	_ = writer.Close()

	return &buf, writer.FormDataContentType()
}

// parseLimited runs ParseForm behind middleware.MaxBodySize, as a server would.
func parseLimited(req *http.Request) error {
	var err error

	handler := middleware.MaxBodySize(testFormLimit)(
		http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			err = httpserver.ParseForm(req, testFormLimit)
		}),
	)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	return err
}

func TestParseForm(t *testing.T) {
	t.Parallel()

	large := strings.Repeat("x", 2*testFormLimit)

	tests := []struct {
		wantErr error
		body    func() (io.Reader, string)
		name    string
		chunked bool
	}{
		{
			name: "url-encoded",
			body: func() (io.Reader, string) { return urlEncodedBody("value") },
		},
		{
			name: "multipart",
			body: func() (io.Reader, string) { return multipartBody("value") },
		},
		{
			name: "other content type",
			body: func() (io.Reader, string) {
				return strings.NewReader(`{"key":"value"}`), "application/json"
			},
		},
		{
			name:    "url-encoded too large",
			body:    func() (io.Reader, string) { return urlEncodedBody(large) },
			wantErr: httpserver.ErrRequestTooLarge,
		},
		{
			name:    "url-encoded too large chunked",
			body:    func() (io.Reader, string) { return urlEncodedBody(large) },
			chunked: true,
			wantErr: httpserver.ErrRequestTooLarge,
		},
		{
			name:    "multipart too large",
			body:    func() (io.Reader, string) { return multipartBody(large) },
			wantErr: httpserver.ErrRequestTooLarge,
		},
		{
			name: "url-encoded malformed",
			body: func() (io.Reader, string) {
				return strings.NewReader("key=%zz"), "application/x-www-form-urlencoded"
			},
			wantErr: httpserver.ErrFormParsing,
		},
		{
			name: "multipart malformed",
			body: func() (io.Reader, string) {
				return strings.NewReader("garbage"), "multipart/form-data; boundary=x"
			},
			wantErr: httpserver.ErrFormParsing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, contentType := tt.body()
			req := newFormRequest(t, "/", body, contentType)

			if tt.chunked {
				req.ContentLength = -1
			}

			err := parseLimited(req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGetPostFormValue_IgnoresQuery(t *testing.T) {
	t.Parallel()

	body, contentType := urlEncodedBody("body")
	req := newFormRequest(t, "/?key=query&other=query", body, contentType)
	require.NoError(t, httpserver.ParseForm(req, testFormLimit))

	var value, other string

	require.NoError(t, httpserver.GetPostFormValue(req, &value, "key", ""))
	require.NoError(t, httpserver.GetPostFormValue(req, &other, "other", "default"))
	assert.Equal(t, "body", value)
	assert.Equal(t, "default", other)
}

func TestGetFormValue_ValidatesDefault(t *testing.T) {
	t.Parallel()

	req := newFormRequest(t, "/", strings.NewReader(""), "application/x-www-form-urlencoded")
	require.NoError(t, httpserver.ParseForm(req, testFormLimit))

	var value string

	err := httpserver.GetFormValue(req, &value, "key", "", validate.NotEmpty)
	require.ErrorIs(t, err, validate.ErrEmpty)

	err = httpserver.GetFormValue(req, &value, "key", "default", validate.NotEmpty)
	require.NoError(t, err)
	assert.Equal(t, "default", value)
}
