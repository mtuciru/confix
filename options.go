package confix

// Option represents a configuration option that can be applied to modify the behavior
// of a configuration instance.
type Option[T any] interface {
	apply(*config[T]) error
}

// afterOptionFunc is a function type that implements the Option interface
// for applying configuration modifications after initialization.
type afterOptionFunc[T any] func(*config[T]) error

func (f afterOptionFunc[T]) apply(cfg *config[T]) error {
	return f(cfg)
}

// WithValidation creates an Option that applies a validation function to the configuration.
// The validation function is called after the configuration is initialized.
func WithValidation[T any](f func(cfg *T) error) Option[T] {
	return afterOptionFunc[T](func(c *config[T]) error {
		return f(c.cfg)
	})
}

// WithWritingConfigToFile creates an Option that writes the configuration to the specified file.
// The file path is provided as a parameter.
func WithWritingConfigToFile[T any](f string) Option[T] {
	return afterOptionFunc[T](func(c *config[T]) error {
		return c.writeToFile(f)
	})
}

// WithSyncingConfigToFiles creates an Option that synchronizes the configuration
// with all registered configuration files.
func WithSyncingConfigToFiles[T any]() Option[T] {
	return afterOptionFunc[T](func(c *config[T]) error {
		return c.writeToFiles()
	})
}
