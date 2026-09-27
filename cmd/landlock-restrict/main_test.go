package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewNetConfig checks the handled rights. Landlock V4 knows only TCP bind and connect, so
// "Net: all" at V4 means exactly those two, and UDP (V9) is never handled.
func TestNewNetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		wants []string
		never []string
		opts  options
	}{
		{
			name:  "bind only",
			opts:  options{restrictBind: true},
			wants: []string{"Landlock V4", "bind_tcp"},
			never: []string{"connect_tcp", "udp"},
		},
		{
			name:  "connect only",
			opts:  options{restrictConnect: true},
			wants: []string{"Landlock V4", "connect_tcp"},
			never: []string{"bind_tcp", "udp"},
		},
		{
			name:  "both",
			opts:  options{restrictBind: true, restrictConnect: true},
			wants: []string{"Landlock V4", "Net: all"},
			never: []string{"udp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, err := newNetConfig(&tt.opts)
			require.NoError(t, err)

			for _, want := range tt.wants {
				assert.Contains(t, config.String(), want)
			}

			for _, never := range tt.never {
				assert.NotContains(t, config.String(), never)
			}
		})
	}
}
