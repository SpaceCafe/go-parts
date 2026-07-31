package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/landlock-lsm/go-landlock/landlock"
)

// Exit codes follow env(1) and chroot(1), so a caller can tell a failure of this wrapper apart from
// the exit status of the command it was asked to run.
const (
	// exitUsage is the status for a malformed command line, matching the flag package.
	exitUsage = 2

	// exitRestrict is the status for restrictions that could not be applied.
	exitRestrict = 125

	// exitExec is the status for a command that was found but could not be executed.
	exitExec = 126

	// exitNotFound is the status for a command that could not be found.
	exitNotFound = 127
)

// listSeparator separates the entries within a single flag value, following the $PATH convention.
// A path containing the separator therefore cannot be expressed, which is the trade-off $PATH makes
// as well.
const listSeparator = string(os.PathListSeparator)

// options holds the restrictions and the command parsed from the arguments.
type options struct {
	command     []string
	bindTCP     portList
	connectTCP  portList
	roDirs      pathList
	roFiles     pathList
	rwDirs      pathList
	rwFiles     pathList
	restrictFS  bool
	restrictNet bool
	strict      bool
}

// pathList collects the values of a repeatable path flag.
type pathList []string

// Set implements flag.Value. An empty value adds no path, which is how a kind of restriction is
// enforced without allowing anything.
func (p *pathList) Set(value string) error {
	*p = append(*p, splitList(value)...)

	return nil
}

func (p *pathList) String() string {
	return strings.Join(*p, listSeparator)
}

// portList collects the values of a repeatable TCP port flag.
type portList []uint16

// Set implements flag.Value. An empty value adds no port, which is how a kind of restriction is
// enforced without allowing anything.
func (p *portList) Set(value string) error {
	for _, entry := range splitList(value) {
		port, err := strconv.ParseUint(entry, 10, 16)
		if err != nil {
			return err
		}

		*p = append(*p, uint16(port))
	}

	return nil
}

func (p *portList) String() string {
	ports := make([]string, len(*p))
	for index, port := range *p {
		ports[index] = strconv.FormatUint(uint64(port), 10)
	}

	return strings.Join(ports, listSeparator)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("landlock-restrict: ")

	opts := parseFlags(os.Args[1:])

	// The lookup runs before the restrictions are in place, because searching PATH afterward would
	// have to stat the very directories the sandbox is there to hide.
	path, err := exec.LookPath(opts.command[0])
	if err != nil {
		fatalf(exitNotFound, "cannot find %q: %v", opts.command[0], err)
	}

	err = applyRestrictions(opts)
	if err != nil {
		fatalf(exitRestrict, "cannot apply restrictions: %v", err)
	}

	// Exec replaces this process, so it only ever returns an error.
	err = syscall.Exec(path, opts.command, os.Environ())
	fatalf(exitExec, "cannot execute %q: %v", path, err)
}

// applyRestrictions enforces the requested rules on the current process.
//
// Filesystem and network rules go into two separate rulesets instead of a single
// landlock.Config.Restrict call: Restrict enforces every kind the config knows about at once, which
// would drag in V9 IPC scoping and would close a kind the caller never named.
func applyRestrictions(opts *options) error {
	config := landlock.V9
	if !opts.strict {
		config = config.BestEffort()
	}

	if opts.restrictFS {
		err := config.RestrictPaths(fsRules(opts)...)
		if err != nil {
			return err
		}
	}

	if opts.restrictNet {
		err := config.RestrictNet(netRules(opts)...)
		if err != nil {
			return err
		}
	}

	return nil
}

// fatalf reports the failure on stderr and exits with the given status.
func fatalf(code int, format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(code)
}

// fsRules turns the path flags into Landlock rules.
//
// The read-write rules ask for WithResolveUnix because ABI V9 counts connect(2) on a pathname UNIX
// socket as filesystem access: without it, a hierarchy handed out for reading and writing would
// still refuse the sockets inside it.
func fsRules(opts *options) []landlock.Rule {
	//nolint:mnd // Number of supported path flags.
	rules := make([]landlock.Rule, 0, 4)

	if len(opts.roDirs) > 0 {
		rules = append(rules, landlock.RODirs(opts.roDirs...))
	}

	if len(opts.roFiles) > 0 {
		rules = append(rules, landlock.ROFiles(opts.roFiles...))
	}

	if len(opts.rwDirs) > 0 {
		rules = append(rules, landlock.RWDirs(opts.rwDirs...).WithResolveUnix())
	}

	if len(opts.rwFiles) > 0 {
		rules = append(rules, landlock.RWFiles(opts.rwFiles...).WithResolveUnix())
	}

	return rules
}

// netRules turns the TCP port flags into Landlock rules.
func netRules(opts *options) []landlock.Rule {
	rules := make([]landlock.Rule, 0, len(opts.bindTCP)+len(opts.connectTCP))

	for _, port := range opts.bindTCP {
		rules = append(rules, landlock.BindTCP(port))
	}

	for _, port := range opts.connectTCP {
		rules = append(rules, landlock.ConnectTCP(port))
	}

	return rules
}

// parseFlags reads the command line and exits with a usage message if it is malformed.
func parseFlags(args []string) *options {
	opts := &options{}
	flags := flag.NewFlagSet("landlock-restrict", flag.ContinueOnError)
	flags.Usage = func() { usage(flags) }

	flags.Var(&opts.roDirs, "ro.dir", "`directory` hierarchy to allow reading and executing in")
	flags.Var(&opts.roFiles, "ro.file", "`file` to allow reading")
	flags.Var(&opts.rwDirs, "rw.dir", "`directory` hierarchy to allow reading and writing in")
	flags.Var(&opts.rwFiles, "rw.file", "`file` to allow reading and writing")
	flags.Var(&opts.bindTCP, "tcp.bind", "TCP `port` to allow bind(2) on")
	flags.Var(&opts.connectTCP, "tcp.connect", "TCP `port` to allow connect(2) on")
	flags.BoolVar(&opts.strict, "strict", false,
		"fail instead of degrading when the kernel supports less than what was asked for")

	err := flags.Parse(args)
	if err != nil {
		// ContinueOnError has already reported the error and the usage on stderr.
		os.Exit(exitUsage)
	}

	// Only a kind of restriction that appears on the command line is enforced, which keeps an empty
	// value meaningful: it enforces the kind with nothing on its allowlist.
	flags.Visit(func(entry *flag.Flag) {
		switch entry.Name {
		case "ro.dir", "ro.file", "rw.dir", "rw.file":
			opts.restrictFS = true
		case "tcp.bind", "tcp.connect":
			opts.restrictNet = true
		}
	})

	opts.command = flags.Args()
	if len(opts.command) == 0 {
		log.Print("no command given")
		flags.Usage()
		os.Exit(exitUsage)
	}

	return opts
}

// splitList splits a flag value into its entries. Empty entries are dropped, so an empty value, a
// trailing separator and a run of separators all stay harmless.
func splitList(value string) []string {
	fields := strings.Split(value, listSeparator)
	entries := make([]string, 0, len(fields))

	for _, entry := range fields {
		if entry != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

// usage prints the synopsis followed by the generated flag list.
func usage(flags *flag.FlagSet) {
	_, _ = fmt.Fprintf(
		flags.Output(),
		"Usage:\n  %s [flags] -- command [args...]\n\n",
		flags.Name(),
	)
	_, _ = fmt.Fprintf(
		flags.Output(),
		"Flags may be repeated or take a %q separated list. A flag that is never given\n"+
			"stays unrestricted; giving it with an empty value denies it altogether.\n\n",
		listSeparator,
	)
	flags.PrintDefaults()
}
