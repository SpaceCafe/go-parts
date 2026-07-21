package procrun

import (
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/spacecafe/go-parts/pkg/validate"
)

const (
	DefaultMaxOpenFiles = 1024
	DefaultMaxProcesses = 1024
)

var _ config.Defaultable = (*Config)(nil)

// Limits defines resource constraints to apply to a process.
type Limits struct {
	// CPU is the maximum CPU time allowed.
	CPU time.Duration `json:"cpu" yaml:"cpu"`

	// Memory is the maximum virtual memory in bytes (RLIMIT_AS).
	Memory typeconv.ByteSize `json:"memory" yaml:"memory"`

	// FileSize is the maximum size of files the process may create (RLIMIT_FSIZE).
	FileSize typeconv.ByteSize `json:"fileSize" yaml:"fileSize"`

	// MaxOpenFiles is the maximum number of open file descriptors (RLIMIT_NOFILE).
	MaxOpenFiles uint64 `json:"maxOpenFiles" yaml:"maxOpenFiles"`

	// MaxProcesses is the maximum number of processes (RLIMIT_NPROC).
	MaxProcesses uint64 `json:"maxProcesses" yaml:"maxProcesses"`

	// CoreDumpSize is the maximum core dump size (RLIMIT_CORE), 0 to disable.
	CoreDumpSize typeconv.ByteSize `json:"coreDumpSize" yaml:"coreDumpSize"`
}

type Config struct {
	Limits Limits `json:"limits" yaml:"limits"`

	AutoCleanup bool `json:"autoCleanup" yaml:"autoCleanup"`
}

func (c *Config) SetDefaults() {
	c.Limits.MaxOpenFiles = DefaultMaxOpenFiles
	c.Limits.MaxProcesses = DefaultMaxProcesses
}

func (c *Config) Validate() error {
	return validate.Validate("cpu", c.Limits.CPU, validate.Positive)
}
