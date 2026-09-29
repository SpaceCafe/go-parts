// Package config loads configuration structs from files and environment variables.
//
// A configuration type implements Validatable, and optionally Defaultable for its defaults. Load
// applies the defaults to a zero-value target, reads the given sources in order (later sources
// override earlier ones) and validates the result. AutoLoad finds the config file itself: the
// path from -config, or the first file in the user config directory or the system config
// directory (and the working directory before them with WithWorkingDir), followed by environment
// variables with a prefix.
//
// Sources are JSONSource, YAMLSource (built with the with_yaml tag) and EnvSource. File sources
// reject unknown keys unless AllowUnknownFields is set. JSONSource reads a time.Duration as a
// string such as "90s" or as integer nanoseconds. EnvSource maps field names to upper-case
// variable names and supports the _FILE suffix for secrets mounted as files.
//
// GenerateTemplate writes a template with the defaults, and never overwrites an existing file.
package config
