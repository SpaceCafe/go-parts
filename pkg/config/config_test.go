package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFileSourceLoad(
	t *testing.T,
	source func(string) config.Source,
	validFile, invalidFile string,
) {
	t.Helper()

	type Config struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}

	type fields struct {
		Path string
	}

	type args struct {
		target any
	}

	tests := []struct {
		args    args
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:   "successful load",
			fields: fields{Path: validFile},
			args:   args{target: &Config{}},
		},
		{
			name:    "file not found",
			fields:  fields{Path: "non-existent"},
			args:    args{target: &Config{}},
			wantErr: true,
		},
		{
			name:    "invalid json content",
			fields:  fields{Path: invalidFile},
			args:    args{target: &Config{}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := source(tt.fields.Path)

			err := s.Load(tt.args.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

var errInvalidPort = errors.New("port must not be zero")

type MockConfig struct {
	Name string `json:"name" yaml:"name"`
	Port int    `json:"port" yaml:"port"`
}

func (c *MockConfig) SetDefaults() {
	c.Name = "default-app"
	c.Port = 8080
}

func (c *MockConfig) Validate() error {
	if c.Port == 0 {
		return errInvalidPort
	}

	return nil
}

// EmptyConfig sets no defaults at all, so New must hand back the plain zero value.
type EmptyConfig struct {
	Port int `json:"port" yaml:"port"`
}

func (c *EmptyConfig) SetDefaults() {}

func (c *EmptyConfig) Validate() error {
	if c.Port == 0 {
		return errInvalidPort
	}

	return nil
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create a test file.
	validFile := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(validFile, []byte(`{"name": "test-app", "port": 8080}`), 0o600)
	require.NoError(t, err)

	t.Setenv("APP_PORT", "9090")

	target := &MockConfig{}
	err = config.AutoLoad(target, "test-app", "APP")
	require.NoError(t, err)
	assert.EqualExportedValues(t, &MockConfig{Name: "test-app", Port: 9090}, target)
}

func TestNew(t *testing.T) {
	t.Parallel()

	// New is generic over the config type, so each case wraps its own instantiation.
	tests := []struct {
		want    any
		newFunc func() any
		name    string
	}{
		{
			name: "defaults are applied",
			newFunc: func() any {
				return config.New[MockConfig]()
			},
			want: &MockConfig{Name: "default-app", Port: 8080},
		},
		{
			name: "empty defaults leave the zero value untouched",
			newFunc: func() any {
				return config.New[EmptyConfig]()
			},
			want: &EmptyConfig{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.newFunc()

			assert.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}

	// Each call must hand out a fresh instance, otherwise callers would share state.
	first := config.New[MockConfig]()
	second := config.New[MockConfig]()
	assert.NotSame(t, first, second)
}

func TestGenerateTemplate(t *testing.T) {
	t.Parallel()

	type args struct {
		target    config.Validatable
		filename  string
		envPrefix string
	}

	tests := []struct {
		compare func(t *testing.T, expected, actual string) bool
		args    args
		name    string
		want    string
	}{
		{
			equalString,
			args{target: &MockConfig{}, filename: "test.env"},
			"env config",
			"NAME=\nPORT=\n",
		},
		{
			equalString,
			args{target: &MockConfig{}, filename: "test.env", envPrefix: "TEST"},
			"env prefixed config",
			"TEST_NAME=\nTEST_PORT=\n",
		},
		{
			equalJSON,
			args{target: &MockConfig{}, filename: "test.json"},
			"json config",
			`{"name":"default-app","port":8080}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, tt.args.filename)

			err := config.GenerateTemplate(tt.args.target, filePath, tt.args.envPrefix)
			require.NoError(t, err)
			require.FileExists(t, filePath)

			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.True(t, tt.compare(t, tt.want, string(content)))
		})
	}
}

func equalString(t *testing.T, expected, actual string) bool {
	t.Helper()

	return assert.Equal(t, expected, actual)
}

func equalJSON(t *testing.T, expected, actual string) bool {
	t.Helper()

	return assert.JSONEq(t, expected, actual)
}
