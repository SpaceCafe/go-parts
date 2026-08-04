package procrun

import (
	"errors"
	"os/exec"
	"time"

	"github.com/spacecafe/go-parts/pkg/config"
	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/spacecafe/go-parts/pkg/validate"
)

const (
	DefaultLandlockBin = "landlock-restrict"
	DefaultPrlimitBin  = "prlimit"
)

var _ config.Defaultable = (*Config)(nil)

// Limits defines resource constraints to apply to a process.
// All fields map to POSIX resource limits set via prlimit(2) on the spawned process.
type Limits struct {
	// CPU is the maximum CPU time a single process may consume (RLIMIT_CPU), counting user and
	// system time together. The kernel rounds it down to whole seconds.
	CPU time.Duration `json:"cpu" yaml:"cpu"`

	// Memory is the maximum virtual address space of a single process in bytes (RLIMIT_AS). This
	// covers mappings rather than resident pages, so it is not an RSS cap and does not limit the
	// process tree in aggregate.
	Memory typeconv.ByteSize `json:"memory" yaml:"memory"`

	// FileSize is the maximum size of any individual file the process may create or extend
	// (RLIMIT_FSIZE). It applies per file, not to the total bytes written.
	FileSize typeconv.ByteSize `json:"fileSize" yaml:"fileSize"`

	// MaxOpenFiles is the maximum number of open file descriptors per process (RLIMIT_NOFILE). Each
	// process has its own descriptor table, so children do not consume the parent's budget.
	MaxOpenFiles uint64 `json:"maxOpenFiles" yaml:"maxOpenFiles"`

	// MaxProcesses is the maximum number of processes per real user ID (RLIMIT_NPROC). The kernel
	// counts every process owned by that user system-wide.
	MaxProcesses uint64 `json:"maxProcesses" yaml:"maxProcesses"`

	// CoreDumpSize is the maximum size of a core dump written for a single process (RLIMIT_CORE),
	// 0 to disable dumps. System settings such as `kernel.core_pattern` still apply on top.
	CoreDumpSize typeconv.ByteSize `json:"coreDumpSize" yaml:"coreDumpSize"`
}

// Restrictions defines the filesystem and network access granted to a process.
// Unlike Limits, these are allowlists rather than quotas. They apply to the spawned process and
// everything it starts.
type Restrictions struct {
	// BindTCP lists the TCP ports the process may bind to. Only consulted when RestrictBindTCP is
	// true.
	BindTCP []int `json:"bindTCP" yaml:"bindTCP"`

	// ConnectTCP lists the TCP ports the process may connect to. Only consulted when
	// RestrictConnectTCP is true.
	ConnectTCP []int `json:"connectTCP" yaml:"connectTCP"`

	// RODirs lists directories the process may read from, including everything below them.
	RODirs []string `json:"roDirs" yaml:"roDirs"`

	// ROFiles lists individual files the process may read from. Use it to grant a single file
	// without opening up its directory.
	ROFiles []string `json:"roFiles" yaml:"roFiles"`

	// RWDirs lists directories the process may read from and write to, including everything below
	// them. The default is the filesystem root, which grants unrestricted access.
	RWDirs []string `json:"rwDirs" yaml:"rwDirs"`

	// RWFiles lists individual files the process may read from and write to. Creating a new file
	// needs write access to its directory, so it takes an RWDirs entry instead.
	RWFiles []string `json:"rwFiles" yaml:"rwFiles"`

	// RestrictBindTCP gates BindTCP. False leaves binding unrestricted, true confines it to the
	// ports in BindTCP, so an empty list denies listening altogether.
	RestrictBindTCP bool `json:"restrictBindTCP" yaml:"restrictBindTCP"`

	// RestrictConnectTCP gates ConnectTCP. False leaves outgoing connections unrestricted, true
	// confines them to the ports in ConnectTCP, so an empty list denies them altogether.
	RestrictConnectTCP bool `json:"restrictConnectTCP" yaml:"restrictConnectTCP"`
}

// Config configures a Runner: the external binaries it shells out to, the restrictions and limits
// applied to spawned processes, and whether their resources are cleaned up automatically.
type Config struct {
	// LandlockBin is the path to the landlock-restrict binary used to apply filesystem
	// restrictions. Resolved via exec.LookPath during Validate.
	LandlockBin string `json:"landlockBin" yaml:"landlockBin"`

	// PrlimitBin is the path to the prlimit binary used to apply resource limits. Resolved via
	// exec.LookPath during Validate.
	PrlimitBin string `json:"prlimitBin" yaml:"prlimitBin"`

	// Restrictions are the filesystem and network allowlists applied to a spawned process.
	Restrictions Restrictions `json:"restrictions" yaml:"restrictions"`

	// Limits are the POSIX resource limits applied to a spawned process.
	Limits Limits `json:"limits" yaml:"limits"`

	// AutoCleanup controls whether a process's resources are released automatically once it exits.
	AutoCleanup bool `json:"autoCleanup" yaml:"autoCleanup"`
}

func (c *Config) SetDefaults() {
	c.LandlockBin = DefaultLandlockBin
	c.PrlimitBin = DefaultPrlimitBin

	c.Restrictions.BindTCP = []int{}
	c.Restrictions.ConnectTCP = []int{}
	c.Restrictions.RODirs = []string{}
	c.Restrictions.ROFiles = []string{}
	c.Restrictions.RWDirs = []string{"/"}
	c.Restrictions.RWFiles = []string{}

	c.Restrictions.RestrictBindTCP = false
	c.Restrictions.RestrictConnectTCP = false
}

func (c *Config) Validate() error {
	return errors.Join(
		validate.Validate("landlock bin", c.LandlockBin, lookPath(&c.LandlockBin)),
		validate.Validate("prlimit bin", c.PrlimitBin, lookPath(&c.PrlimitBin)),
		validate.Validate(
			"bind tcp restrictions",
			c.Restrictions.BindTCP,
			validate.NotNilSlice,
			validate.Elements[int](validate.Port),
		),
		validate.Validate(
			"connect tcp restrictions",
			c.Restrictions.ConnectTCP,
			validate.NotNilSlice,
			validate.Elements[int](validate.Port),
		),
		validate.Validate(
			"ro dirs restrictions",
			c.Restrictions.RODirs,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty),
		),
		validate.Validate(
			"ro files restrictions",
			c.Restrictions.ROFiles,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty),
		),
		validate.Validate(
			"rw dirs restrictions",
			c.Restrictions.RWDirs,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty),
		),
		validate.Validate(
			"rw files restrictions",
			c.Restrictions.RWFiles,
			validate.NotNilSlice,
			validate.Elements[string](validate.NotEmpty),
		),
	)
}

// lookPath returns a validation function that resolves value to an absolute
// executable path via exec.LookPath and stores the result in *target.
func lookPath(target *string) func(value string) error {
	return func(value string) (err error) {
		if value == "" {
			return nil
		}

		*target, err = exec.LookPath(value)

		return
	}
}
