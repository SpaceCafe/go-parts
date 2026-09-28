//go:build linux

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

func TestSandboxArgs_PrlimitBeforeLandlock(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.SetDefaults()

	args := sandboxArgs(cfg)

	assert.Equal(t, cfg.PrlimitBin, args[0])
	assert.Equal(t, "--", args[len(args)-1])
	assert.Equal(t, prlimitArgs(cfg), args[:len(prlimitArgs(cfg))])
	assert.Equal(t, landlockArgs(cfg), args[len(prlimitArgs(cfg)):])
}

func TestSandboxArgs_ExtraRWDirs(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Restrictions.RWDirs = []string{"/srv/data"}

	args := sandboxArgs(cfg, "/tmp/work")

	assert.Contains(t, args, "-rw.dir=/srv/data"+listSeparator+"/tmp/work")
	assert.Equal(t, []string{"/srv/data"}, cfg.Restrictions.RWDirs, "config must stay unchanged")
}

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
