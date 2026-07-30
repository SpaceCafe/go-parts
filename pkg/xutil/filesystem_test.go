package xutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacecafe/go-parts/pkg/xutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func(t *testing.T) (src, dest string)
		name    string
		wantErr bool
	}{
		{
			name: "successful copy",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				return src, filepath.Join(dir, "dest")
			},
		},
		{
			name: "missing source",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()

				return filepath.Join(dir, "missing"), filepath.Join(dir, "dest")
			},
			wantErr: true,
		},
		{
			name: "destination directory does not exist",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				return src, filepath.Join(dir, "missing", "dest")
			},
			wantErr: true,
		},
		{
			name: "destination file already exists",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				dir := t.TempDir()
				src := filepath.Join(dir, "src")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

				dest := filepath.Join(dir, "dest")
				require.NoError(
					t,
					os.WriteFile(dest, []byte("stale content that is longer"), 0o600),
				)

				return src, dest
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src, dest := tt.setup(t)
			err := xutil.CopyFile(src, dest)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			want, err := os.ReadFile(src)
			require.NoError(t, err)

			got, err := os.ReadFile(dest)
			require.NoError(t, err)

			assert.Equal(t, want, got)
		})
	}
}
