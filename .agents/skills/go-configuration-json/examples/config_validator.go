package config

import (
	"fmt"
	"strconv"
	"strings"
)

// =============================================================================
// Fail-Fast Multi-Error Validation Engine
// =============================================================================

type ValidationError struct {
	Field   string
	Message string
}

type Validator struct {
	errors []ValidationError
}

func NewValidator() *Validator {
	return &Validator{errors: make([]ValidationError, 0)}
}

func (v *Validator) Add(field, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

func (v *Validator) Error() string {
	var sb strings.Builder
	for _, err := range v.errors {
		sb.WriteString(fmt.Sprintf("  - %-25s : %s\n", err.Field, err.Message))
	}
	return sb.String()
}

// Validate executes all invariant checks across configuration sections.
func (v *Validator) Validate(cfg *Configuration) error {
	v.validateServer(&cfg.Server)
	v.validateDatabase(&cfg.Database)
	v.validateAuth(&cfg.Auth)

	if v.HasErrors() {
		return fmt.Errorf("%s", v.Error())
	}
	return nil
}

func (v *Validator) validateServer(s *ServerSettings) {
	port, err := strconv.Atoi(s.Port)
	if err != nil || port < 1 || port > 65535 {
		v.Add("server.port", "must be a valid TCP port between 1 and 65535")
	}

	if s.ReadTimeout.Duration() <= 0 {
		v.Add("server.read_timeout", "must be strictly positive (e.g. 5s)")
	}
	if s.WriteTimeout.Duration() <= 0 {
		v.Add("server.write_timeout", "must be strictly positive (e.g. 10s)")
	}
	if s.ShutdownTimeout.Duration() <= 0 {
		v.Add("server.shutdown_timeout", "must be strictly positive (e.g. 15s)")
	}

	if s.ShutdownTimeout.Duration() <= s.DrainDuration.Duration() {
		v.Add("server.shutdown_timeout", fmt.Sprintf("must be strictly greater than drain_duration (%v <= %v)",
			s.ShutdownTimeout.Duration(), s.DrainDuration.Duration()))
	}
}

func (v *Validator) validateDatabase(d *DatabaseSettings) {
	if d.Driver != "postgres" && d.Driver != "memory" {
		v.Add("database.driver", "supported drivers are 'postgres' or 'memory'")
	}

	if d.Driver == "postgres" {
		if strings.TrimSpace(d.Host) == "" {
			v.Add("database.host", "host is required when driver is postgres")
		}
		if strings.TrimSpace(d.Name) == "" {
			v.Add("database.name", "database name is required")
		}
		if strings.TrimSpace(d.User) == "" {
			v.Add("database.user", "database user is required")
		}
	}
}

func (v *Validator) validateAuth(a *AuthSettings) {
	if len(a.JWTSecret) < 32 {
		v.Add("auth.jwt_secret", "must be at least 32 characters long for cryptographic security")
	}
}
