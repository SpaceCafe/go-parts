package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const defaultFilePermission = 0o600

var (
	ErrInvalidTarget  = errors.New("config: invalid target")
	ErrConfigNotFound = errors.New("config: config not found")
	ErrInvalidConfig  = errors.New("config: invalid config")
	ErrValidation     = errors.New("config: validation failed")
	ErrTemplateCreate = errors.New("config: cannot create template file")
)

// Defaultable allows a configuration struct to set its own default values.
type Defaultable interface {
	SetDefaults()
}

// Validatable ensures that the configuration struct provides a validation method.
type Validatable interface {
	Validate() error
}

// Source defines a configuration source.
type Source interface {
	GenerateTemplate(target any, output io.Writer) error
	Load(target any) error
}

type pointerDefaultable[T any] interface {
	*T
	Defaultable
}

// AutoLoad loads target from the first config file found and from environment variables with
// envPrefix, then validates it. It reads its options from os.Args without parsing flag.CommandLine:
//
//   - -config <path> names the config file; it must exist.
//   - -generate-template[=<path>] writes a template (default config.tmpl.json) and exits.
//
// Both flags are also registered on flag.CommandLine unless already defined, so an application that
// calls flag.Parse itself (before or after AutoLoad) accepts them. AutoLoad never calls flag.Parse.
//
// An empty envPrefix is allowed, but fields whose derived names collide with common system
// variables are rejected, see EnvSource.
func AutoLoad(target Validatable, name, envPrefix string, opts ...Option) error {
	settings := &options{}
	for _, opt := range opts {
		opt(settings)
	}

	registerAutoLoadFlags()

	args, err := parseAutoLoadArgs(os.Args[1:])
	if err != nil {
		return err
	}

	if args.generateTemplate {
		err = GenerateTemplate(target, args.templatePath, envPrefix)
		if err != nil {
			return err
		}

		os.Exit(0)
	}

	sources := []Source{}

	source, err := findConfigSource(name, args.configPath, settings.allowUnknownFields)
	if err != nil {
		return err
	}

	if source != nil {
		sources = append(sources, source)
	}

	// Add environment variable source
	sources = append(sources, &EnvSource{Prefix: envPrefix})

	return Load(target, sources...)
}

// GenerateTemplate writes a configuration template for the target and writes it to the specified file.
// It never overwrites an existing file, so passing the path of a live config cannot replace it with
// defaults. If writing the template fails, the partial file is removed.
func GenerateTemplate(target Validatable, filename, envPrefix string) (err error) {
	err = validatePointerToStruct(target)
	if err != nil {
		return err
	}

	if filename == "" {
		filename = defaultTemplatePath
	}

	// Pick the source before touching the file, so an unsupported suffix leaves nothing behind.
	var source Source

	if strings.HasSuffix(filename, ".env") {
		source = &EnvSource{Prefix: envPrefix}
	} else {
		source, err = sourceFromSuffix(filename, false)
		if err != nil {
			return err
		}
	}

	// Apply defaults if the target implements Defaultable
	if defaultable, ok := target.(Defaultable); ok {
		defaultable.SetDefaults()
	}

	filename = filepath.Clean(filename)

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, defaultFilePermission)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemplateCreate, err)
	}

	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			_ = os.Remove(filename)
		}
	}()

	return source.GenerateTemplate(target, file)
}

// Load loads configuration from multiple sources and validates the result.
// This is simpler than using a Loader struct for this straightforward operation.
//
// If target implements Defaultable and is still the zero value, SetDefaults is applied first. A
// target that already holds values (for example from New, with some fields changed afterwards) is
// left as is, so those values are kept unless a source overrides them. A target with only some
// fields set by hand therefore gets no defaults; call SetDefaults or use New before setting them.
func Load(target Validatable, sources ...Source) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	if defaultable, ok := target.(Defaultable); ok {
		if reflect.ValueOf(target).Elem().IsZero() {
			defaultable.SetDefaults()
		} else {
			slog.Debug(
				"config: target is not the zero value, defaults not applied",
				"type", fmt.Sprintf("%T", target),
			)
		}
	}

	for _, s := range sources {
		err = s.Load(target)
		if err != nil {
			return err
		}
	}

	err = target.Validate()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrValidation, err)
	}

	return nil
}

func New[T any, PT pointerDefaultable[T]]() *T {
	config := PT(new(T))
	config.SetDefaults()

	return config
}

// findConfigSource returns the source for the config file to load. An explicit configPath must
// exist; its error is returned instead of falling back to another file. Without one, the first
// existing file from configPaths is used, and only files that do not exist are skipped, so a
// permission error is reported rather than silently loading a different config. A nil Source
// without error means no file was found.
//
//nolint:ireturn // Returns whichever Source matches the file suffix.
func findConfigSource(name, configPath string, allowUnknownFields bool) (Source, error) {
	if configPath != "" {
		_, err := os.Stat(configPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
		}

		return sourceFromSuffix(configPath, allowUnknownFields)
	}

	for _, filePath := range configPaths(name) {
		_, err := os.Stat(filePath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
		}

		return sourceFromSuffix(filePath, allowUnknownFields)
	}

	return nil, nil //nolint:nilnil // No config file is a valid outcome; env vars still apply.
}

// configPaths generates a list of potential configuration file paths for the given application name.
func configPaths(name string) []string {
	filePaths := []string{
		filepath.Join(".", name+".json"),
		filepath.Join(".", name+".yml"),
		filepath.Join(".", name+".yaml"),
		filepath.Join(".", "config.json"),
		filepath.Join(".", "config.yml"),
		filepath.Join(".", "config.yaml"),
		filepath.Join(".", "config", name+".json"),
		filepath.Join(".", "config", name+".yml"),
		filepath.Join(".", "config", name+".yaml"),
	}

	userDir, err := os.UserConfigDir()
	if err == nil {
		filePaths = append(filePaths,
			filepath.Join(userDir, name+".json"),
			filepath.Join(userDir, name+".yml"),
			filepath.Join(userDir, name+".yaml"),
			filepath.Join(userDir, name, "config.json"),
			filepath.Join(userDir, name, "config.yml"),
			filepath.Join(userDir, name, "config.yaml"))
	}

	sysDir := systemConfigDir()
	filePaths = append(filePaths,
		filepath.Join(sysDir, name, "config.json"),
		filepath.Join(sysDir, name, "config.yml"),
		filepath.Join(sysDir, name, "config.yaml"),
		filepath.Join(sysDir, name+".json"),
		filepath.Join(sysDir, name+".yml"),
		filepath.Join(sysDir, name+".yaml"))

	return filePaths
}

// sourceFromSuffix determines the configuration source (JSON or YAML) based on the file's suffix.
// Returns an appropriate Source or an error if the file format is unsupported.
//
//nolint:ireturn // Factory function must return an interface type to support multiple source implementations.
func sourceFromSuffix(filename string, allowUnknownFields bool) (Source, error) {
	if strings.HasSuffix(filename, ".json") {
		return &JSONSource{Path: filename, AllowUnknownFields: allowUnknownFields}, nil
	}

	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		return newYAMLSource(filename, allowUnknownFields)
	}

	return nil, fmt.Errorf("%w: unsupported file format: %s", ErrInvalidConfig, filename)
}

// systemConfigDir returns the system-wide configuration directory. On Unix-like systems this is
// /etc, on Windows it is %ProgramData%.
func systemConfigDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("ProgramData"); dir != "" {
			return dir
		}

		return `C:\ProgramData`
	}

	return "/etc"
}

// validatePointerToStruct ensures the target is a non-nil pointer to a struct.
func validatePointerToStruct(target any) error {
	if target == nil {
		return fmt.Errorf("%w: target cannot be nil", ErrInvalidTarget)
	}

	valueOf := reflect.ValueOf(target)
	if valueOf.Kind() != reflect.Pointer {
		return fmt.Errorf(
			"%w: target must be a pointer, got %T",
			ErrInvalidTarget,
			target,
		)
	}

	if valueOf.IsNil() {
		return fmt.Errorf("%w: target pointer cannot be nil", ErrInvalidTarget)
	}

	if valueOf.Elem().Kind() != reflect.Struct {
		return fmt.Errorf(
			"%w: target must be a pointer to struct, got pointer to %s",
			ErrInvalidTarget,
			valueOf.Elem().Kind(),
		)
	}

	return nil
}
