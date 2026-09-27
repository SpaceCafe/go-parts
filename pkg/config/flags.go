package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

const (
	configFlag           = "config"
	generateTemplateFlag = "generate-template"
	defaultTemplatePath  = "config.tmpl.json"
)

var ErrInvalidArgs = errors.New("config: invalid command-line arguments")

// autoLoadArgs holds the command-line options AutoLoad understands.
type autoLoadArgs struct {
	configPath       string
	templatePath     string
	generateTemplate bool
}

// parseAutoLoadArgs scans args for -config and -generate-template (with one or two dashes) and
// ignores every other argument, so it works alongside the application's own flags without touching
// flag.CommandLine. -config takes a value ("-config=x" or "-config x"). -generate-template takes an
// optional path, only in the "=" form; "true" and "false" are accepted for compatibility with the
// former boolean flag. Scanning stops at "--".
func parseAutoLoadArgs(args []string) (autoLoadArgs, error) {
	var result autoLoadArgs

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}

		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") {
			continue
		}

		switch name {
		case configFlag:
			if !hasValue {
				if i+1 >= len(args) {
					return result, fmt.Errorf("%w: -%s needs a value", ErrInvalidArgs, configFlag)
				}

				i++
				value = args[i]
			}

			result.configPath = value
		case generateTemplateFlag:
			switch {
			case !hasValue || value == "true":
				result.generateTemplate = true
				result.templatePath = defaultTemplatePath
			case value == "false":
				result.generateTemplate = false
			default:
				result.generateTemplate = true
				result.templatePath = value
			}
		}
	}

	return result, nil
}

// registerAutoLoadFlags defines -config and -generate-template on flag.CommandLine unless the
// application already did, so an application that calls flag.Parse accepts them and lists them in
// -h. The values parsed there are not used; AutoLoad scans os.Args itself.
func registerAutoLoadFlags() {
	if flag.Lookup(configFlag) == nil {
		flag.String(configFlag, "", "path to config file")
	}

	if flag.Lookup(generateTemplateFlag) == nil {
		flag.Var(
			new(optionalValue),
			generateTemplateFlag,
			"write a configuration template to the given path (default "+defaultTemplatePath+") and exit",
		)
	}
}

// optionalValue is a flag.Value that may be given without a value, like a boolean flag.
type optionalValue string

func (v *optionalValue) IsBoolFlag() bool { return true }

func (v *optionalValue) Set(value string) error {
	*v = optionalValue(value)

	return nil
}

func (v *optionalValue) String() string {
	if v == nil {
		return ""
	}

	return string(*v)
}
