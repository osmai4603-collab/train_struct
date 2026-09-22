package config

// Option represents a configuration functional option to customize Engine behavior.
type Option func(*Engine)

// WithConfigFile configures the file path for the JSON configuration store.
func WithConfigFile(path string) Option {
	return func(e *Engine) {
		e.pipeline.configFile = path
	}
}

// WithCLIArgs passes command-line arguments to the resolution pipeline.
func WithCLIArgs(args []string) Option {
	return func(e *Engine) {
		e.pipeline.cliArgs = args
	}
}

// WithEnvLookup injects a custom environment lookup function for hermetic testing.
func WithEnvLookup(lookup func(key string) (string, bool)) Option {
	return func(e *Engine) {
		e.pipeline.envLookup = lookup
	}
}

// WithDisableEnv disables environment variable overrides.
func WithDisableEnv() Option {
	return func(e *Engine) {
		e.pipeline.disableEnv = true
	}
}

// WithRuntimeOverrides attaches a programmatic override function (Layer 5).
func WithRuntimeOverrides(fn func(*Configuration)) Option {
	return func(e *Engine) {
		if fn != nil {
			e.pipeline.overrides = append(e.pipeline.overrides, fn)
		}
	}
}

// WithValidationRule registers a custom invariant rule into the ValidationEngine.
func WithValidationRule(rule ValidationRule) Option {
	return func(e *Engine) {
		if rule != nil {
			e.validator.RegisterRule(rule)
		}
	}
}
