package typeconv_test

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/stretchr/testify/assert"
)

func TestToFuncs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		convert func(string) string
		value   string
		want    string
	}{
		{
			name:    "snake_case",
			convert: typeconv.ToSnakeCase,
			value:   "HTTPServerConfig",
			want:    "http_server_config",
		},
		{
			name:    "kebab-case",
			convert: typeconv.ToKebabCase,
			value:   "HTTPServerConfig",
			want:    "http-server-config",
		},
		{
			name:    "flatcase",
			convert: typeconv.ToFlatCase,
			value:   "HTTPServerConfig",
			want:    "httpserverconfig",
		},
		{
			name:    "lower case",
			convert: typeconv.ToLowerCase,
			value:   "HTTPServerConfig",
			want:    "http server config",
		},
		{
			name:    "CONSTANT_CASE",
			convert: typeconv.ToConstantCase,
			value:   "HTTPServerConfig",
			want:    "HTTP_SERVER_CONFIG",
		},
		{
			name:    "COBOL-CASE",
			convert: typeconv.ToCobolCase,
			value:   "HTTPServerConfig",
			want:    "HTTP-SERVER-CONFIG",
		},
		{
			name:    "UPPERFLATCASE",
			convert: typeconv.ToUpperFlatCase,
			value:   "HTTPServerConfig",
			want:    "HTTPSERVERCONFIG",
		},
		{
			name:    "UPPER CASE",
			convert: typeconv.ToUpperCase,
			value:   "HTTPServerConfig",
			want:    "HTTP SERVER CONFIG",
		},
		{
			name:    "Ada_Case",
			convert: typeconv.ToAdaCase,
			value:   "HTTPServerConfig",
			want:    "Http_Server_Config",
		},
		{
			name:    "Train-Case",
			convert: typeconv.ToTrainCase,
			value:   "HTTPServerConfig",
			want:    "Http-Server-Config",
		},
		{
			name:    "PascalCase",
			convert: typeconv.ToPascalCase,
			value:   "HTTPServerConfig",
			want:    "HttpServerConfig",
		},
		{
			name:    "Title Case",
			convert: typeconv.ToTitleCase,
			value:   "HTTPServerConfig",
			want:    "Http Server Config",
		},
		{
			name:    "camelCase",
			convert: typeconv.ToCamelCase,
			value:   "HTTPServerConfig",
			want:    "httpServerConfig",
		},

		{name: "empty value", convert: typeconv.ToSnakeCase, value: "", want: ""},
		{name: "separators only", convert: typeconv.ToCamelCase, value: "__", want: ""},
		{name: "single word", convert: typeconv.ToCamelCase, value: "ID", want: "id"},
		{name: "trailing acronym", convert: typeconv.ToKebabCase, value: "userID", want: "user-id"},
		{
			name:    "digits kept",
			convert: typeconv.ToConstantCase,
			value:   "parseInt64",
			want:    "PARSE_INT64",
		},
		{
			name:    "digit before upper",
			convert: typeconv.ToSnakeCase,
			value:   "utf8Decode",
			want:    "utf8_decode",
		},
		{
			name:    "mixed delimiters",
			convert: typeconv.ToPascalCase,
			value:   " parse--HTTPHeader_v2 ",
			want:    "ParseHttpHeaderV2",
		},
		{
			name:    "punctuation dropped",
			convert: typeconv.ToTitleCase,
			value:   "hello, world!",
			want:    "Hello World",
		},
		{name: "uncased script", convert: typeconv.ToSnakeCase, value: "テスト", want: "テスト"},
		{
			name:    "unicode letters",
			convert: typeconv.ToSnakeCase,
			value:   "straßeName",
			want:    "straße_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.convert(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
