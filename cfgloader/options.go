package cfgloader

// Options holds configuration options for MustLoad.
type Options struct {
	// Silent disables all config logging to stdout when set to true.
	Silent bool
}

// Option is a functional option for configuring MustLoad behavior.
type Option func(*Options)

// WithSilent disables config logging to stdout.
func WithSilent() Option {
	return func(o *Options) {
		o.Silent = true
	}
}
