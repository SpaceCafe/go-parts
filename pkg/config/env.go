package config

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/spacecafe/go-parts/pkg/typeconv"
)

var (
	_ Source = (*EnvSource)(nil)

	ErrConversion      = errors.New("config: failed to convert environment variable to field type")
	ErrReservedEnvName = errors.New(
		"config: field maps to a reserved environment variable, set a prefix or an env tag",
	)
	ErrEnvNameCollision = errors.New(
		"config: fields map to colliding environment variables, set an env tag on one of them",
	)

	// ReservedEnvNames lists common system variables that an unprefixed field name must not map
	// onto. It is not exhaustive: it covers names that realistically collide with config fields.
	//
	//nolint:gochecknoglobals // Read-only set of names.
	ReservedEnvNames = map[string]struct{}{
		"HOME": {}, "PATH": {}, "USER": {}, "USERNAME": {}, "LOGNAME": {}, "SHELL": {},
		"HOST": {}, "HOSTNAME": {}, "PWD": {}, "OLDPWD": {}, "LANG": {}, "LANGUAGE": {},
		"LC_ALL": {}, "TERM": {}, "TMPDIR": {}, "TEMP": {}, "TMP": {}, "TZ": {},
		"DISPLAY": {}, "EDITOR": {}, "PAGER": {}, "MAIL": {}, "SHLVL": {}, "UID": {},
		"GID": {}, "USERPROFILE": {}, "APPDATA": {}, "COMPUTERNAME": {}, "SYSTEMROOT": {},
		"OS": {},
	}
)

// EnvSource loads configuration from environment variables. Prefix may be empty, but then top-level
// fields map onto unprefixed names, so a field named Home or Path would read HOME or PATH. With an
// empty prefix, a field name that derives one of the ReservedEnvNames is therefore rejected with
// ErrReservedEnvName. An explicit env tag is taken as deliberate and is not checked.
//
// Every field also reads NAME_FILE (see lookupEnv). Two fields whose names collide, either exactly or
// because one is the other plus _FILE (Cert and CertFile), are rejected with ErrEnvNameCollision
// before anything is loaded.
type EnvSource struct {
	Prefix string
}

// GenerateTemplate writes one NAME= line per field, optional pointer sections included. It only
// inspects the type of target, so it neither reads the environment nor changes target.
func (s EnvSource) GenerateTemplate(target any, output io.Writer) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	names, err := s.envNames(reflect.TypeOf(target).Elem())
	if err != nil {
		return err
	}

	var errs []error

	for _, name := range names {
		_, err1 := output.Write([]byte(name))
		_, err2 := output.Write([]byte("=\n"))
		errs = append(errs, err1, err2)
	}

	return errors.Join(errs...)
}

func (s EnvSource) Load(target any) error {
	err := validatePointerToStruct(target)
	if err != nil {
		return err
	}

	valueOf := reflect.ValueOf(target).Elem()

	_, err = s.envNames(valueOf.Type())
	if err != nil {
		return err
	}

	return s.loadStruct(valueOf, strings.ToUpper(s.Prefix))
}

// envNames returns the env names of all leaf fields of typeOf and rejects reserved and colliding
// names, without reading the environment.
func (s EnvSource) envNames(typeOf reflect.Type) ([]string, error) {
	var names []string

	err := collectEnvNames(typeOf, strings.ToUpper(s.Prefix), map[reflect.Type]bool{}, &names)
	if err != nil {
		return nil, err
	}

	err = checkEnvNameCollisions(names)
	if err != nil {
		return nil, err
	}

	return names, nil
}

// collectEnvNames appends the env names of all leaf fields of typeOf to names, following the same
// rules as loadStruct but without reading the environment, so optional pointer-to-struct sections
// are included. visiting holds the struct types on the current path, so a self-referencing type is
// not expanded endlessly.
func collectEnvNames(
	typeOf reflect.Type,
	prefix string,
	visiting map[reflect.Type]bool,
	names *[]string,
) error {
	visiting[typeOf] = true
	defer delete(visiting, typeOf)

	for fieldType := range typeOf.Fields() {
		if !fieldType.IsExported() {
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "-" {
			continue
		}

		envName := createEnvName(prefix, fieldType.Name, envTag)

		if nested, ok := nestedStruct(fieldType.Type); ok {
			if visiting[nested] {
				continue
			}

			err := collectEnvNames(nested, envName, visiting, names)
			if err != nil {
				return err
			}

			continue
		}

		err := checkReservedEnvName(prefix, envTag, fieldType.Name, envName)
		if err != nil {
			return err
		}

		*names = append(*names, envName)
	}

	return nil
}

// nestedStruct returns the struct type that a field of type typeOf groups other fields in,
// following one pointer, or false when typeOf is a leaf that loads from a single variable.
// time.Time and structs whose pointer implements encoding.TextUnmarshaler hold one value, as
// typeconv converts them.
func nestedStruct(typeOf reflect.Type) (reflect.Type, bool) {
	if typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}

	if typeOf.Kind() != reflect.Struct ||
		typeOf == reflect.TypeFor[time.Time]() ||
		reflect.PointerTo(typeOf).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		return nil, false
	}

	return typeOf, true
}

// checkEnvNameCollisions rejects names that occur twice, and a name N next to N_FILE: lookupEnv reads
// N_FILE as the file holding the value of N, so setting it meant for the second field would also
// load that file into the first.
func checkEnvNameCollisions(names []string) error {
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		if _, dup := seen[name]; dup {
			return fmt.Errorf("%w: %s", ErrEnvNameCollision, name)
		}

		seen[name] = struct{}{}
	}

	for _, name := range names {
		if _, dup := seen[name+"_FILE"]; dup {
			return fmt.Errorf("%w: %s and %s_FILE", ErrEnvNameCollision, name, name)
		}
	}

	return nil
}

// hasEnvWithPrefix checks if any environment variable with the given prefix exists.
func (s EnvSource) hasEnvWithPrefix(prefix string) bool {
	prefix += "_"
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			return true
		}
	}

	return false
}

// loadStruct recursively loads environment variables into struct fields.
func (s EnvSource) loadStruct(valueOf reflect.Value, prefix string) error {
	typeOf := valueOf.Type()

	for i := range valueOf.NumField() {
		field := valueOf.Field(i)
		fieldType := typeOf.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "-" {
			continue
		}

		// Build the environment variable name
		envName := createEnvName(prefix, fieldType.Name, envTag)

		_, nested := nestedStruct(field.Type())

		// Handle nested structs recursively
		if nested && field.Kind() == reflect.Struct {
			err := s.loadStruct(field, envName)
			if err != nil {
				return err
			}

			continue
		}

		// Handle pointers to structs
		//nolint:nestif // Required for optional nested struct initialization and loading.
		if nested && field.Kind() == reflect.Pointer {
			// Initialize nil pointer if environment variable exists
			if s.hasEnvWithPrefix(envName) {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}

				err := s.loadStruct(field.Elem(), envName)
				if err != nil {
					return err
				}
			}

			continue
		}

		err := loadField(field, envName)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkReservedEnvName rejects an envName that is one of the ReservedEnvNames when it was derived
// from the field name without a prefix. Prefixed names cannot collide, and an explicit env tag is
// taken as deliberate. It is only called for leaf fields; a struct field just contributes a prefix.
// collectEnvNames calls it for every leaf field, before anything is loaded.
func checkReservedEnvName(prefix, envTag, fieldName, envName string) error {
	if prefix != "" || envTag != "" {
		return nil
	}

	if _, reserved := ReservedEnvNames[envName]; reserved {
		return fmt.Errorf("%w: %s -> %s", ErrReservedEnvName, fieldName, envName)
	}

	return nil
}

// loadField sets field from the environment variable envName, if it is set.
func loadField(field reflect.Value, envName string) error {
	envValue, exists, err := lookupEnv(envName)
	if err != nil || !exists {
		return err
	}

	err = typeconv.Default.Convert(field, envValue)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConversion, err)
	}

	return nil
}

// createEnvName generates an environment variable name using the provided prefix, field name, and optional env tag.
// It converts camel case field names to uppercase with underscores and includes the prefix if provided.
func createEnvName(prefix, fieldName, envTag string) string {
	var result strings.Builder

	if prefix != "" {
		result.WriteString(prefix)
		result.WriteRune('_')
	}

	if envTag != "" {
		result.WriteString(strings.ToUpper(envTag))

		return result.String()
	}

	runes := []rune(fieldName)
	for i, char := range runes {
		if i > 0 && unicode.IsUpper(char) {
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(runes[i-1]) || nextIsLower {
				result.WriteRune('_')
			}
		}

		result.WriteRune(unicode.ToUpper(char))
	}

	return result.String()
}

// lookupEnv returns the value for envName. If envName_FILE is set, the value is read from that file
// (the Docker and Kubernetes secrets convention), and a read failure is an error rather than a
// silent fallback to envName. File contents lose a single trailing line break, which editors and
// secret mounts commonly add; everything else, including plain variables, is used unchanged.
func lookupEnv(envName string) (value string, exists bool, err error) {
	filePath, exists := os.LookupEnv(envName + "_FILE")
	if exists {
		data, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return "", false, fmt.Errorf("%w: %s_FILE: %w", ErrConfigNotFound, envName, err)
		}

		value = strings.TrimSuffix(string(data), "\n")
		value = strings.TrimSuffix(value, "\r")

		return value, true, nil
	}

	value, exists = os.LookupEnv(envName)

	return value, exists, nil
}
