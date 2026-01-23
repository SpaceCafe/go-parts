package httpserver_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPServer_Start(t *testing.T) {
	t.Parallel()

	certFile, keyFile := generateTestCert(t)

	tests := []struct {
		ctx     context.Context //nolint:containedctx // Required for testing.
		wantErr error
		server  *httpserver.HTTPServer
		name    string
	}{
		{
			name:    "nil context",
			server:  httpserver.New(&httpserver.Config{}),
			ctx:     nil,
			wantErr: httpserver.ErrInvalidContext,
		},
		{
			name:   "cancelled context",
			server: httpserver.New(&httpserver.Config{}),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			wantErr: httpserver.ErrInvalidContext,
		},
		{
			name:    "server startup without TLS succeeds",
			server:  httpserver.New(&httpserver.Config{}, httpserver.WithLogger(&mockLogger{})),
			ctx:     context.Background(),
			wantErr: nil,
		},
		{
			name: "server startup with TLS succeeds",
			server: httpserver.New(
				&httpserver.Config{Port: 8081, CertFile: certFile, KeyFile: keyFile},
				httpserver.WithLogger(&mockLogger{}),
			),
			ctx:     context.Background(),
			wantErr: nil,
		},
		{
			name: "server startup fails immediately",
			server: httpserver.New(
				&httpserver.Config{Port: 99999},
				httpserver.WithLogger(&mockLogger{}),
			),
			ctx:     context.Background(),
			wantErr: &net.AddrError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.server.Start(tt.ctx)
			{
				var errCase0 *net.AddrError
				switch {
				case tt.wantErr == nil:
					require.NoError(t, err)
				case errors.As(tt.wantErr, &errCase0):
					var addrErr *net.AddrError
					require.ErrorAs(t, err, &addrErr)
				default:
					require.ErrorIs(t, err, tt.wantErr)
				}
			}

			if err == nil {
				assert.NoError(t, tt.server.Server.Shutdown(context.Background()))
			}
		})
	}
}

type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...any) {}
func (m *mockLogger) Error(_ string, _ ...any) {}
func (m *mockLogger) Info(_ string, _ ...any)  {}
func (m *mockLogger) Warn(_ string, _ ...any)  {}

// generateTestCert creates a self-signed certificate for testing.
func generateTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	// Create and store private key.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	keyFile = filepath.Join(t.TempDir(), "key.pem")
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)

	defer func() {
		_ = keyOut.Close()
	}()

	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)

	err = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	require.NoError(t, err)

	// Create and store self-signed certificate.
	template := x509.Certificate{
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey,
	)
	require.NoError(t, err)

	// Write certificate to temp file
	certFile = filepath.Join(t.TempDir(), "cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)

	defer func() {
		_ = certOut.Close()
	}()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)

	return certFile, keyFile
}
