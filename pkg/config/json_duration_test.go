package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type durationLimits struct {
	CPU time.Duration `json:"cpu"`
}

type durationEmbedded struct {
	Idle time.Duration `json:"idle"`
}

// durationOuter promotes the fields of durationEmbedded.
type durationOuter struct {
	durationEmbedded
}

type durationConfig struct {
	Retries  map[string]time.Duration `json:"retries"`
	Limits   *durationLimits          `json:"limits"`
	Steps    []time.Duration          `json:"steps"`
	Timeout  time.Duration            `json:"timeout"`
	Size     typeconv.ByteSize        `json:"size"`
	Untagged time.Duration
}

// loadDurationJSON writes content to a JSON file and loads it into a durationConfig.
func loadDurationJSON(t *testing.T, content string) (*durationConfig, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	target := &durationConfig{}

	return target, config.JSONSource{Path: path}.Load(target)
}

func TestJSONSource_Load_Durations(t *testing.T) {
	t.Parallel()

	target, err := loadDurationJSON(t, `{
		"timeout": "1m30s",
		"limits": {"cpu": "2s"},
		"steps": ["1s", 2000000000],
		"retries": {"first": "500ms"},
		"UNTAGGED": "3s",
		"size": "2MiB"
	}`)
	require.NoError(t, err)

	assert.Equal(t, 90*time.Second, target.Timeout)
	assert.Equal(t, 2*time.Second, target.Limits.CPU)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, target.Steps)
	assert.Equal(t, map[string]time.Duration{"first": 500 * time.Millisecond}, target.Retries)
	assert.Equal(t, 3*time.Second, target.Untagged, "case-insensitive key")
	assert.Equal(t, typeconv.ByteSize(2*1024*1024), target.Size, "custom unmarshaler untouched")
}

func TestJSONSource_Load_DurationEmbedded(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"idle": "1h"}`), 0o600))

	target := &durationOuter{}
	require.NoError(t, config.JSONSource{Path: path}.Load(target))

	assert.Equal(t, time.Hour, target.Idle)
}

func TestJSONSource_Load_DurationNanos(t *testing.T) {
	t.Parallel()

	target, err := loadDurationJSON(t, `{"timeout": 90000000000}`)
	require.NoError(t, err)

	assert.Equal(t, 90*time.Second, target.Timeout)
}

func TestJSONSource_Load_InvalidDuration(t *testing.T) {
	t.Parallel()

	_, err := loadDurationJSON(t, `{"limits": {"cpu": "soon"}}`)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	require.ErrorIs(t, err, config.ErrInvalidDuration)
	require.ErrorContains(t, err, "limits.cpu")
}

func TestJSONSource_Load_DurationKeepsChecks(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"unknown field": `{"timeout": "1s", "timeuot": "2s"}`,
		"trailing data": `{"timeout": "1s"} {"timeout": "2s"}`,
		"syntax error":  `{"timeout": "1s",}`,
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadDurationJSON(t, content)

			require.ErrorIs(t, err, config.ErrInvalidConfig)
		})
	}
}

func TestJSONSource_GenerateTemplate_Durations(t *testing.T) {
	t.Parallel()

	source := &durationConfig{
		Timeout: 90 * time.Second,
		Limits:  &durationLimits{CPU: 2 * time.Second},
		Steps:   []time.Duration{time.Second},
	}

	var output bytes.Buffer

	require.NoError(t, config.JSONSource{}.GenerateTemplate(source, &output))

	assert.Contains(t, output.String(), `"timeout":"1m30s"`)
	assert.Contains(t, output.String(), `"cpu":"2s"`)
	assert.Contains(t, output.String(), `"steps":["1s"]`)

	// The template loads back into the same values.
	target, err := loadDurationJSON(t, output.String())
	require.NoError(t, err)
	assert.Equal(t, source.Timeout, target.Timeout)
	assert.Equal(t, source.Limits.CPU, target.Limits.CPU)
	assert.Equal(t, source.Steps, target.Steps)
}
