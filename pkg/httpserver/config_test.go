package httpserver_test

import (
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_TLS(t *testing.T) {
	t.Parallel()

	certFile, keyFile := generateTestCert(t)
	missingFile := filepath.Join(t.TempDir(), "missing.pem")

	tests := []struct {
		wantErr  error
		name     string
		certFile string
		keyFile  string
	}{
		{name: "no TLS"},
		{name: "cert and key", certFile: certFile, keyFile: keyFile},
		{name: "cert only", certFile: certFile, wantErr: httpserver.ErrIncompleteTLS},
		{name: "key only", keyFile: keyFile, wantErr: httpserver.ErrIncompleteTLS},
		{
			name:     "missing cert file",
			certFile: missingFile,
			keyFile:  keyFile,
			wantErr:  validate.ErrPathNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &httpserver.Config{}
			cfg.SetDefaults()
			cfg.CertFile = tt.certFile
			cfg.KeyFile = tt.keyFile

			err := cfg.Validate()
			if tt.wantErr == nil {
				assert.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
