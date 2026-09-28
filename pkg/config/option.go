package config

// Option is a functional option for configuring AutoLoad.
type Option func(*options)

// options holds the settings that Option values apply.
type options struct {
	allowUnknownFields bool
	workingDir         bool
}

// WithAllowUnknownFields makes AutoLoad accept keys in the config file that no field of the target
// matches, see JSONSource.AllowUnknownFields. By default they fail loading.
func WithAllowUnknownFields() Option {
	return func(opts *options) {
		opts.allowUnknownFields = true
	}
}

// WithWorkingDir makes AutoLoad also search the working directory (./ and ./config/), ahead of the
// user and system config directories. It is off by default: a binary started from an untrusted
// directory, such as a shared /tmp or a cloned repository, would otherwise load a config file an
// attacker placed there. Use it for local development, or pass -config to name the file.
func WithWorkingDir() Option {
	return func(opts *options) {
		opts.workingDir = true
	}
}
