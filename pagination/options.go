package pagination

const (
	defaultPageSize = 20
	defaultMaxSize  = 100
)

// Options configures pagination behavior.
type Options struct {
	MaxPageSize int
}

type Option func(*Options)

func WithMaxPageSize(maxSize int) Option {
	return func(o *Options) {
		o.MaxPageSize = maxSize
	}
}

func defaultOptions() Options {
	return Options{MaxPageSize: defaultMaxSize}
}
