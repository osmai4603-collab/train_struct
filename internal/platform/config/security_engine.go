package config

import (
	"net/url"
	"strings"
)

// =============================================================================
// Security & Redaction Engine
// =============================================================================

// SecurityEngine handles secret protection, credential masking, and sanitized logging representations.
type SecurityEngine struct{}

// NewSecurityEngine creates a new security and redaction engine instance.
func NewSecurityEngine() *SecurityEngine {
	return &SecurityEngine{}
}

// Redact returns a sanitized copy of Configuration where sensitive secrets
// (database passwords, tokens, JWT keys, connection string credentials) are masked with "***".
func (s *SecurityEngine) Redact(cfg *Configuration) *Configuration {
	if cfg == nil {
		return nil
	}

	clone := cfg.Clone()

	if clone.Database.Password != "" {
		clone.Database.Password = "***"
	}
	if clone.Database.DatabaseURL != "" {
		clone.Database.DatabaseURL = s.MaskURL(clone.Database.DatabaseURL)
	}
	if clone.Auth.JWTSecret != "" {
		clone.Auth.JWTSecret = "***"
	}

	return clone
}

// MaskURL sanitizes database connection URLs by replacing sensitive credentials with "***".
// If the URL cannot be safely parsed, it returns "***" to prevent credential leakage.
func (s *SecurityEngine) MaskURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "***"
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(username, "***")
		}
	}

	return strings.ReplaceAll(parsed.String(), "%2A%2A%2A", "***")
}

// Redacted provides a convenience method directly on Configuration.
func (c *Configuration) Redacted() *Configuration {
	engine := NewSecurityEngine()
	return engine.Redact(c)
}
