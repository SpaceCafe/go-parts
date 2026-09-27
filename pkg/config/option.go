package config

// Option is a functional option for configuring AutoLoad.
type Option func(*options)

// options holds the settings that Option values apply.
type options struct {
	allowUnknownFields bool
}

// WithAllowUnknownFields makes AutoLoad accept keys in the config file that no field of the target
// matches, see JSONSource.AllowUnknownFields. By default they fail loading.
func WithAllowUnknownFields() Option {
	return func(opts *options) {
		opts.allowUnknownFields = true
	}
}
