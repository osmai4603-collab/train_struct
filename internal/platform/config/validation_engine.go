package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// =============================================================================
// Fail-Fast Multi-Error Validation Engine
// =============================================================================

// ValidationError describes a single configuration invariant violation.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationReport accumulates multiple validation errors into a single diagnostic report.
type ValidationReport struct {
	Errors []ValidationError `json:"errors"`
}

// Add appends a new validation error to the report.
func (r *ValidationReport) Add(field, message string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if any violations have been registered.
func (r *ValidationReport) HasErrors() bool {
	return len(r.Errors) > 0
}

// Error formats all violations into a structured, readable multi-line diagnostic report.
func (r *ValidationReport) Error() string {
	if len(r.Errors) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("configuration validation failed (%d error%s detected):\n",
		len(r.Errors), ternary(len(r.Errors) == 1, "", "s")))

	for _, err := range r.Errors {
		sb.WriteString(fmt.Sprintf("  - %-28s : %s\n", err.Field, err.Message))
	}
	return sb.String()
}

// ValidationRule defines an invariant check function.
type ValidationRule func(cfg *Configuration, report *ValidationReport)

// ValidationEngine orchestrates and executes validation rules against a configuration.
type ValidationEngine struct {
	rules []ValidationRule
}

// NewValidationEngine instantiates an engine pre-configured with standard production rules.
func NewValidationEngine(customRules ...ValidationRule) *ValidationEngine {
	e := &ValidationEngine{
		rules: make([]ValidationRule, 0),
	}

	// Register built-in standard rules
	e.RegisterRule(validateServerInvariants)
	e.RegisterRule(validateLoggerInvariants)
	e.RegisterRule(validateDatabaseInvariants)
	e.RegisterRule(validateAuthInvariants)

	for _, rule := range customRules {
		e.RegisterRule(rule)
	}

	return e
}

// RegisterRule attaches an invariant check rule to the validation pipeline.
func (e *ValidationEngine) RegisterRule(rule ValidationRule) {
	if rule != nil {
		e.rules = append(e.rules, rule)
	}
}

// Validate executes all registered rules and returns an aggregated error report if any violation occurred.
func (e *ValidationEngine) Validate(cfg *Configuration) error {
	if cfg == nil {
		report := &ValidationReport{}
		report.Add("configuration", "cannot be nil")
		return report
	}

	report := &ValidationReport{}
	for _, rule := range e.rules {
		rule(cfg, report)
	}

	if report.HasErrors() {
		return report
	}
	return nil
}

// -----------------------------------------------------------------------------
// Built-in Validation Rules
// -----------------------------------------------------------------------------

func validateServerInvariants(cfg *Configuration, report *ValidationReport) {
	s := &cfg.Server

	port, err := strconv.Atoi(s.Port)
	if err != nil || port < 1 || port > 65535 {
		report.Add("server.port", fmt.Sprintf("must be a valid TCP port between 1 and 65535 (got %q)", s.Port))
	}

	if s.ReadTimeout.Duration() <= 0 {
		report.Add("server.read_timeout", "must be strictly positive (e.g. '5s')")
	}
	if s.WriteTimeout.Duration() <= 0 {
		report.Add("server.write_timeout", "must be strictly positive (e.g. '10s')")
	}
	if s.ShutdownTimeout.Duration() <= 0 {
		report.Add("server.shutdown_timeout", "must be strictly positive (e.g. '15s')")
	}
	if s.DrainDuration.Duration() < 0 {
		report.Add("server.drain_duration", "cannot be negative")
	}

	if s.ShutdownTimeout.Duration() > 0 && s.DrainDuration.Duration() > 0 {
		if s.ShutdownTimeout.Duration() <= s.DrainDuration.Duration() {
			report.Add("server.shutdown_timeout",
				fmt.Sprintf("must be strictly greater than drain_duration (%v <= %v)",
					s.ShutdownTimeout.Duration(), s.DrainDuration.Duration()))
		}
	}
}

func validateLoggerInvariants(cfg *Configuration, report *ValidationReport) {
	l := &cfg.Logger

	if strings.TrimSpace(l.Level) != "" {
		switch strings.ToLower(strings.TrimSpace(l.Level)) {
		case "debug", "info", "warn", "warning", "error":
		default:
			report.Add("logger.level", fmt.Sprintf("unknown log level %q (supported: debug, info, warn, error)", l.Level))
		}
	}

	if strings.TrimSpace(l.LogDir) == "" && l.ToFile {
		report.Add("logger.log_dir", "cannot be empty when log file persistence is enabled")
	}
}

func validateDatabaseInvariants(cfg *Configuration, report *ValidationReport) {
	d := &cfg.Database

	if d.Driver != "postgres" && d.Driver != "memory" {
		report.Add("database.driver", fmt.Sprintf("unsupported driver %q; supported: 'postgres', 'memory'", d.Driver))
	}

	if d.Driver == "postgres" {
		if d.DatabaseURL != "" {
			u, err := url.Parse(d.DatabaseURL)
			if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
				report.Add("database.database_url", "must be a valid postgres URI (e.g. 'postgres://user:pass@host:5432/db')")
			}
		} else {
			if strings.TrimSpace(d.Host) == "" {
				report.Add("database.host", "host is required when driver is postgres")
			}
			if strings.TrimSpace(d.Name) == "" {
				report.Add("database.name", "database name is required")
			}
			if strings.TrimSpace(d.User) == "" {
				report.Add("database.user", "database user is required")
			}
			dbPort, err := strconv.Atoi(d.Port)
			if err != nil || dbPort < 1 || dbPort > 65535 {
				report.Add("database.port", fmt.Sprintf("must be a valid TCP port between 1 and 65535 (got %q)", d.Port))
			}
		}

		if d.MaxOpenConns <= 0 {
			report.Add("database.max_open_conns", "must be greater than 0")
		}
		if d.MaxIdleConns < 0 {
			report.Add("database.max_idle_conns", "cannot be negative")
		}
		if d.MaxIdleConns > d.MaxOpenConns {
			report.Add("database.max_idle_conns", "cannot exceed max_open_conns")
		}
		if d.MinConns < 0 {
			report.Add("database.min_conns", "cannot be negative")
		}
		if d.MinConns > d.MaxOpenConns {
			report.Add("database.min_conns", "cannot exceed max_open_conns")
		}
	}
}

func validateAuthInvariants(cfg *Configuration, report *ValidationReport) {
	a := &cfg.Auth

	if len(a.JWTSecret) < 32 {
		report.Add("auth.jwt_secret",
			fmt.Sprintf("must be at least 32 characters long for cryptographic security (length: %d)", len(a.JWTSecret)))
	}
	if a.TokenTTL.Duration() <= 0 {
		report.Add("auth.token_ttl", "must be strictly positive (e.g. '24h')")
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
