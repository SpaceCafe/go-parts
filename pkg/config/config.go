package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

var (
	ErrInvalidTarget  = errors.New("config: invalid target")
	ErrConfigNotFound = errors.New("config: config not found")
	ErrInvalidConfig  = errors.New("config: invalid config")
	ErrValidation     = errors.New("config: validation failed")
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
	Load(target any) error
}

type pointerDefaultable[T any] interface {
	*T
	Defaultable
}

func AutoLoad(target Validatable, name, envPrefix string) error {
	var (
		sources []Source
		source  Source
	)

	for _, filePath := range configPaths(name) {
		if filePath == "" || filePath == "." || filePath == "./" {
			continue
		}

		_, err := os.Stat(filePath)
		if err == nil {
			source, err = sourceFromSuffix(filePath)
			if err == nil {
				sources = append(sources, source)

				break
			}

			return err
		}
	}

	// Add environment variable source
	sources = append(sources, &EnvSource{Prefix: envPrefix})

	return Load(target, sources...)
}

// Load loads configuration from multiple sources and validates the result.
// This is simpler than using a Loader struct for this straightforward operation.
func Load(target Validatable, sources ...Source) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	// Apply defaults if the target implements Defaultable
	if defaultable, ok := target.(Defaultable); ok {
		defaultable.SetDefaults()
	}

	for _, s := range sources {
		err = s.Load(target)
		if err != nil {
			return err
		}
	}

	err = target.Validate()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}

	return nil
}

func New[T any, PT pointerDefaultable[T]]() *T {
	config := PT(new(T))
	config.SetDefaults()

	return config
}

// configPaths generates a list of potential configuration file paths for the given application name.
func configPaths(name string) []string {
	configPath := flag.String("config", "", "path to config file")

	flag.Parse()

	filePaths := []string{
		filepath.Clean(*configPath),
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
func sourceFromSuffix(filename string) (Source, error) {
	if strings.HasSuffix(filename, ".json") {
		return &JSONSource{Path: filename}, nil
	}

	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		return newYAMLSource(filename)
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
