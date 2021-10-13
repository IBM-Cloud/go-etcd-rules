package lock

const (
	unknown = "unknown"
)

type options struct {
	// The pattern that was used to build to lock key
	pattern string
	// The method to provide context
	method string
}

func buildOptions(opts ...Option) options {
	os := options{
		pattern: unknown,
		method:  unknown,
	}
	for _, opt := range opts {
		opt(&os)
	}
	return os
}

// Option instances are used to provide optional arguments to
// the RuleLock.Lock method.
type Option func(lo *options)

// LockPattern is used to specify the pattern that was used to
// build the lock key for metric tracking purposes.
func LockPattern(pattern string) Option {
	return func(lo *options) {
		lo.pattern = pattern
	}
}

func LockMethod(method string) Option {
	return func(lo *options) {
		lo.method = method
	}
}
