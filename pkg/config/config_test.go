package config_test

import (
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
		name    string
		fields  fields
		args    args
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

type MockConfig struct {
	Name string `json:"name" yaml:"name"`
	Port int    `json:"port" yaml:"port"`
}

func (c *MockConfig) Validate() error {
	return nil
}

func TestLoad(t *testing.T) {
	// Create a test file.
	validFile := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(validFile, []byte(`{"name": "test-app", "port": 8080}`), 0o600)
	require.NoError(t, err)

	t.Setenv("APP_PORT", "9090")

	target := &MockConfig{}
	err = config.Load(target, config.JSONSource{Path: validFile}, config.EnvSource{Prefix: "APP"})
	require.NoError(t, err)
	assert.EqualExportedValues(t, &MockConfig{Name: "test-app", Port: 9090}, target)
}
