package httpserver_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
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
			server:  httpserver.New(testConfig(0, "", "")),
			ctx:     nil,
			wantErr: httpserver.ErrInvalidContext,
		},
		{
			name:   "cancelled context",
			server: httpserver.New(testConfig(0, "", "")),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			wantErr: httpserver.ErrInvalidContext,
		},
		{
			name:    "server startup without TLS succeeds",
			server:  httpserver.New(testConfig(0, "", ""), httpserver.WithLogger(&mockLogger{})),
			ctx:     context.Background(),
			wantErr: nil,
		},
		{
			name: "server startup with TLS succeeds",
			server: httpserver.New(
				testConfig(8081, certFile, keyFile),
				httpserver.WithLogger(&mockLogger{}),
			),
			ctx:     context.Background(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.server.Start(tt.ctx)
			require.ErrorIs(t, err, tt.wantErr)

			if err == nil {
				assert.NoError(t, tt.server.Server.Shutdown(context.Background()))
			}
		})
	}
}

func TestHTTPServer_TLSHandshake(t *testing.T) {
	t.Parallel()

	certFile, keyFile := generateTestCert(t)

	server := httpserver.New(
		testConfig(8444, certFile, keyFile),
		httpserver.WithLogger(&mockLogger{}),
	)
	server.Server.Handler = http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusNoContent)
	})

	require.NoError(t, server.Start(context.Background()))
	t.Cleanup(func() { _ = server.Server.Shutdown(context.Background()) })

	certPEM, err := os.ReadFile(certFile)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			ServerName: "localhost",
			MinVersion: tls.VersionTLS12,
		},
	}}

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"https://127.0.0.1:8444/",
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestHTTPServer_Start_InvalidKeyPair(t *testing.T) {
	t.Parallel()

	// Files that exist pass Validate in New, so the unparsable key pair only fails in Start.
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, []byte("not a certificate"), 0o600))
	require.NoError(t, os.WriteFile(keyFile, []byte("not a key"), 0o600))

	server := httpserver.New(
		testConfig(8445, certFile, keyFile),
		httpserver.WithLogger(&mockLogger{}),
	)

	require.Error(t, server.Start(context.Background()))
}

func TestNew_InvalidConfigPanics(t *testing.T) {
	t.Parallel()

	tests := map[string]*httpserver.Config{
		"port out of range": testConfig(99999, "", ""),
		"missing key pair":  testConfig(8080, "/nonexistent/cert.pem", "/nonexistent/key.pem"),
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				err, ok := recover().(error)
				require.True(t, ok, "New must panic with an error")
				assert.ErrorIs(t, err, config.ErrValidation)
			}()

			httpserver.New(cfg)
		})
	}
}

func TestHTTPServer_Start_PortInUse(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	server := httpserver.New(
		testConfig(addr.Port, "", ""),
		httpserver.WithLogger(&mockLogger{}),
	)

	require.ErrorIs(t, server.Start(context.Background()), syscall.EADDRINUSE)
}

func TestNew_ListenAddress(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"127.0.0.1": "127.0.0.1:8080",
		"localhost": "localhost:8080",
		"::1":       "[::1]:8080",
	}

	for host, wantAddr := range tests {
		cfg := testConfig(8080, "", "")
		cfg.Host = host

		server := httpserver.New(cfg)
		assert.Equal(t, wantAddr, server.Server.Addr, "host %q", host)
	}
}

func TestHTTPServer_Stop(t *testing.T) {
	t.Parallel()

	server := httpserver.New(testConfig(8446, "", ""), httpserver.WithLogger(&mockLogger{}))
	require.NoError(t, server.Start(context.Background()))

	require.NoError(t, server.Stop(context.Background()))
}

func TestHTTPServer_StartAfterStop(t *testing.T) {
	t.Parallel()

	server := httpserver.New(testConfig(8447, "", ""), httpserver.WithLogger(&mockLogger{}))
	require.NoError(t, server.Start(context.Background()))
	require.NoError(t, server.Stop(context.Background()))

	require.ErrorIs(t, server.Start(context.Background()), httpserver.ErrServerStopped)
}

func TestHTTPServer_StartAfterServerShutdown(t *testing.T) {
	t.Parallel()

	server := httpserver.New(testConfig(8448, "", ""), httpserver.WithLogger(&mockLogger{}))
	require.NoError(t, server.Server.Shutdown(context.Background()))

	require.ErrorIs(t, server.Start(context.Background()), httpserver.ErrServerStopped)
}

// testConfig returns a valid Config listening on port, with TLS enabled when certFile and keyFile
// are set. Port 0 picks a free port.
func testConfig(port int, certFile, keyFile string) *httpserver.Config {
	cfg := &httpserver.Config{}
	cfg.SetDefaults()
	cfg.Port = port
	cfg.CertFile = certFile
	cfg.KeyFile = keyFile

	return cfg
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
