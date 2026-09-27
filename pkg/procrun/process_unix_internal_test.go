//go:build unix

package procrun

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLandlockArgs_Strict(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.SetDefaults()

	assert.NotContains(t, landlockArgs(cfg), "-strict")

	cfg.Restrictions.Strict = true

	args := landlockArgs(cfg)
	assert.Contains(t, args, "-strict")
	assert.Equal(t, "--", args[len(args)-1])
}

func TestApplyArguments_PrlimitBeforeLandlock(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.SetDefaults()
	cfg.LandlockBin = "/usr/bin/landlock-restrict"
	cfg.PrlimitBin = "/usr/bin/prlimit"

	runner := New(cfg, WithLogger(discardLogger{}))

	assert.Equal(t, "/usr/bin/prlimit", runner.args[0])
	assert.Equal(t, "--", runner.args[len(runner.args)-1])
	assert.Equal(t, prlimitArgs(cfg), runner.args[:len(prlimitArgs(cfg))])
	assert.Equal(t, landlockArgs(cfg), runner.args[len(prlimitArgs(cfg)):])
}

type discardLogger struct{}

func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Error(string, ...any) {}
func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Warn(string, ...any)  {}

func TestPrlimitArgs_CPU(t *testing.T) {
	t.Parallel()

	for cpu, want := range map[time.Duration]string{
		400 * time.Millisecond:  "--cpu=1",
		time.Second:             "--cpu=1",
		2900 * time.Millisecond: "--cpu=2",
	} {
		cfg := &Config{}
		cfg.SetDefaults()
		cfg.Limits.CPU = cpu

		assert.Contains(t, prlimitArgs(cfg), want, "cpu %s", cpu)
	}
}
