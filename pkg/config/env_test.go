package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacecafe/go-parts/pkg/config"
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
		RefSub  *SubConfig
		hidden  string
		Skip    string `env:"-"`
		Name    string `env:"NAME"`
		CCBin   string
		Tags    []string  `env:"TAGS"`
		Options []int     `env:"OPTIONS"`
		Sub     SubConfig `env:"SUB"`
		Port    int       `env:"PORT"`
	}

	type fields struct {
		Prefix string
	}

	type args struct {
		target any
		env    map[string]string
	}

	tests := []struct {
		args    args
		name    string
		fields  fields
		want    Config
		wantErr bool
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
					"APP_CC_BIN":                      "gcc",
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
				CCBin:   "gcc",
			},
		},
		{
			name:   "successful load without prefix",
			fields: fields{Prefix: ""},
			args: args{
				target: &Config{},
				env: map[string]string{
					"NAME":   "standalone",
					"PORT":   "9000",
					"CC_BIN": "gcc",
				},
			},
			want: Config{Name: "standalone", Port: 9000, CCBin: "gcc"},
		},
		{
			name:   "unreadable _FILE does not fall back",
			fields: fields{Prefix: ""},
			args: args{
				target: &Config{},
				env: map[string]string{
					"PORT":      "9000",
					"PORT_FILE": "non-existent",
				},
			},
			wantErr: true,
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

func TestEnvSource_Load_Values(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("  s3cr3t \n"), 0o600))

	t.Setenv("VAL_SECRET_FILE", secretFile)
	t.Setenv("VAL_PADDED", "  keep spaces  ")

	var target struct {
		Secret string
		Padded string
	}

	require.NoError(t, config.EnvSource{Prefix: "VAL"}.Load(&target))

	// A single trailing newline is removed from files; everything else is kept as is.
	assert.Equal(t, "  s3cr3t ", target.Secret)
	assert.Equal(t, "  keep spaces  ", target.Padded)
}

//nolint:paralleltest // Uses t.Setenv.
func TestEnvSource_Load_ReservedNames(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	t.Run("field name colliding without prefix", func(t *testing.T) {
		var target struct{ Home string }

		require.ErrorIs(t, config.EnvSource{}.Load(&target), config.ErrReservedEnvName)
	})

	t.Run("explicit env tag is allowed", func(t *testing.T) {
		var target struct {
			Dir string `env:"HOME"`
		}

		require.NoError(t, config.EnvSource{}.Load(&target))
		assert.Equal(t, "/home/someone", target.Dir)
	})

	t.Run("prefix avoids the collision", func(t *testing.T) {
		var target struct{ Home string }

		require.NoError(t, config.EnvSource{Prefix: "APP"}.Load(&target))
		assert.Empty(t, target.Home)
	})

	t.Run("nested field is prefixed by its parent", func(t *testing.T) {
		var target struct{ Server struct{ Home string } }

		require.NoError(t, config.EnvSource{}.Load(&target))
	})
}

func TestEnvSource_Load_FileSuffixCollision(t *testing.T) {
	t.Parallel()

	type TLS struct{ Cert string }

	tests := map[string]any{
		"sibling fields": &struct {
			Cert     string
			CertFile string
		}{},
		"across nesting levels": &struct {
			TLS         TLS
			TLSCertFile string
		}{},
		"optional nested section": &struct {
			TLS         *TLS
			TLSCertFile string
		}{},
		"same name twice": &struct {
			Cert  string
			Other string `env:"CERT"`
		}{},
	}

	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(
				t,
				config.EnvSource{Prefix: "APP"}.Load(target),
				config.ErrEnvNameCollision,
			)
		})
	}
}

func TestEnvSource_Load_SelfReferencingStruct(t *testing.T) {
	t.Parallel()

	type Node struct {
		Next *Node
		Name string
	}

	require.NoError(t, config.EnvSource{Prefix: "APP"}.Load(&Node{}))
}

func TestEnvSource_GenerateTemplate(t *testing.T) {
	t.Setenv("TPL_NAME", "from-env")

	type TLS struct{ CertFile string }

	target := &struct {
		TLS  *TLS
		Name string
	}{}

	var output strings.Builder

	require.NoError(t, config.EnvSource{Prefix: "TPL"}.GenerateTemplate(target, &output))

	// The optional section is listed although it is nil, and nothing is loaded into the target.
	assert.Equal(t, "TPL_TLS_CERT_FILE=\nTPL_NAME=\n", output.String())
	assert.Nil(t, target.TLS)
	assert.Empty(t, target.Name)
}
