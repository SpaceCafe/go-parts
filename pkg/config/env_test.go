package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpaceCafe/go-parts/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvSource_Load(t *testing.T) {
	// Create a test file.
	envFile := filepath.Join(t.TempDir(), "config_value")
	err := os.WriteFile(envFile, []byte(`8080`), 0o600)
	require.NoError(t, err)

	type SubConfig struct {
		Value             string `env:"VALUE"`
		NotAnnotatedValue int
	}

	type Config struct {
		hidden  string
		Skip    string    `env:"-"`
		Name    string    `env:"NAME"`
		Port    int       `env:"PORT"`
		Tags    []string  `env:"TAGS"`
		Options []int     `env:"OPTIONS"`
		Sub     SubConfig `env:"SUB"`
		RefSub  *SubConfig
	}

	type fields struct {
		Prefix string
	}

	type args struct {
		target any
		env    map[string]string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    Config
	}{
		{
			name:   "successful load with prefix",
			fields: fields{Prefix: "APP"},
			args: args{
				target: &Config{},
				env: map[string]string{
					"APP_NAME":                        "test-app",
					"APP_PORT_FILE":                   envFile,
					"APP_TAGS":                        "prod,web,go",
					"APP_OPTIONS":                     "1, 2, 3",
					"APP_SUB_VALUE":                   "nested-payload",
					"APP_SUB_NOT_ANNOTATED_VALUE":     "42",
					"APP_REF_SUB_NOT_ANNOTATED_VALUE": "42",
				},
			},
			want: Config{
				hidden:  "",
				Name:    "test-app",
				Port:    8080,
				Tags:    []string{"prod", "web", "go"},
				Options: []int{1, 2, 3},
				Sub:     SubConfig{Value: "nested-payload", NotAnnotatedValue: 42},
				RefSub:  &SubConfig{NotAnnotatedValue: 42},
			},
		},
		{
			name:   "successful load without prefix",
			fields: fields{Prefix: ""},
			args: args{
				target: &Config{},
				env: map[string]string{
					"NAME":      "standalone",
					"PORT":      "9000",
					"PORT_FILE": "non-existent",
				},
			},
			want: Config{Name: "standalone", Port: 9000},
		},
		{
			name:    "invalid target (not a pointer)",
			fields:  fields{Prefix: "APP"},
			args:    args{target: Config{}},
			wantErr: true,
		},
		{
			name:   "invalid value type in env",
			fields: fields{Prefix: "APP"},
			args: args{
				target: &Config{},
				env:    map[string]string{"APP_PORT": "not-a-number"},
			},
			wantErr: true,
		},
		{
			name:   "invalid value type in sub",
			fields: fields{Prefix: "APP"},
			args: args{
				target: &Config{},
				env:    map[string]string{"APP_SUB_NOT_ANNOTATED_VALUE": "not-a-number"},
			},
			wantErr: true,
		},
		{
			name:   "invalid value type in ref sub",
			fields: fields{Prefix: "APP"},
			args: args{
				target: &Config{},
				env:    map[string]string{"APP_REF_SUB_NOT_ANNOTATED_VALUE": "not-a-number"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.args.env {
				t.Setenv(k, v)
			}

			s := config.EnvSource{
				Prefix: tt.fields.Prefix,
			}

			err := s.Load(tt.args.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.EqualExportedValues(t, &tt.want, tt.args.target)
			}
		})
	}
}
