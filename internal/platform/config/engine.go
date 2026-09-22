package config

import (
	"fmt"
	"sync/atomic"
)

// =============================================================================
// Central Configuration Engine
// =============================================================================

// Engine serves as the central orchestrator coordinating all configuration sub-engines:
// Pipeline, Validation, Security/Redaction, Persistence, and Schema Introspection.
type Engine struct {
	pipeline    *PipelineEngine
	validator   *ValidationEngine
	security    *SecurityEngine
	persistence *PersistenceEngine
	schema      *SchemaEngine
	current     atomic.Pointer[Configuration]
}

// NewEngine instantiates a fully wired Configuration Engine with default sub-engines and applies functional options.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		pipeline:    NewPipelineEngine(),
		validator:   NewValidationEngine(),
		security:    NewSecurityEngine(),
		persistence: NewPersistenceEngine(),
		schema:      NewSchemaEngine(),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}

	return e
}

// Load executes the resolution pipeline across all layers (Defaults < File < Env < CLI < Overrides)
// and runs fail-fast multi-error validation. It returns an error if any invariant is violated.
func (e *Engine) Load() (*Configuration, error) {
	cfg, err := e.pipeline.Resolve()
	if err != nil {
		return nil, fmt.Errorf("configuration resolution failed: %w", err)
	}

	if err := e.validator.Validate(cfg); err != nil {
		return nil, err
	}

	e.current.Store(cfg)
	return cfg, nil
}

// Current returns the active, validated configuration in an atomic, non-blocking manner.
// If Load() has not yet been invoked, it returns DefaultConfiguration().
func (e *Engine) Current() *Configuration {
	cfg := e.current.Load()
	if cfg == nil {
		return DefaultConfiguration()
	}
	return cfg
}

// Reload atomically re-resolves and re-validates the configuration without interrupting active operations.
func (e *Engine) Reload() (*Configuration, error) {
	return e.Load()
}

// Validate executes all invariant checks against the provided configuration.
func (e *Engine) Validate(cfg *Configuration) error {
	return e.validator.Validate(cfg)
}

// Redact produces a sanitized copy of Configuration suitable for logging and display,
// ensuring secrets and credentials are masked with "***".
func (e *Engine) Redact(cfg *Configuration) *Configuration {
	return e.security.Redact(cfg)
}

// Save atomically writes the configuration to disk in JSON format with strict 0600 permissions.
func (e *Engine) Save(path string, cfg *Configuration) error {
	return e.persistence.Save(path, cfg)
}

// LoadFile unmarshals configuration data directly from a JSON file on disk.
func (e *Engine) LoadFile(path string, target *Configuration) error {
	return e.persistence.LoadFile(path, target)
}

// ExtractSpecs inspects struct tags to generate metadata specifications for each property.
func (e *Engine) ExtractSpecs(v interface{}) []FieldSpec {
	return e.schema.ExtractSpecs(v)
}

// GenerateEnvTemplate generates a complete, commented .env.example string for default configuration.
func (e *Engine) GenerateEnvTemplate() string {
	specs := e.schema.ExtractSpecs(DefaultConfiguration())
	return e.schema.GenerateEnvTemplate(specs)
}

// WriteEnvExample creates an .env.example file on disk.
func (e *Engine) WriteEnvExample(path string) error {
	return e.schema.WriteEnvExample(path, DefaultConfiguration())
}

// Sub-engine accessors for direct specialized access if needed:

// Pipeline returns the underlying PipelineEngine.
func (e *Engine) Pipeline() *PipelineEngine { return e.pipeline }

// Validator returns the underlying ValidationEngine.
func (e *Engine) Validator() *ValidationEngine { return e.validator }

// Security returns the underlying SecurityEngine.
func (e *Engine) Security() *SecurityEngine { return e.security }

// Persistence returns the underlying PersistenceEngine.
func (e *Engine) Persistence() *PersistenceEngine { return e.persistence }

// Schema returns the underlying SchemaEngine.
func (e *Engine) Schema() *SchemaEngine { return e.schema }

// Load is a package-level convenience function to resolve and validate configuration in one call.
func Load(opts ...Option) (*Configuration, error) {
	engine := NewEngine(opts...)
	return engine.Load()
}
