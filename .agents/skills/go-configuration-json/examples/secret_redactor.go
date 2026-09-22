package config

import (
	"fmt"
	"net/url"
	"strings"
)

// =============================================================================
// Secret Redaction & URL Credential Masking
// =============================================================================

// Redacted creates a shallow copy of Configuration with credentials masked with ***.
func Redacted(cfg *Configuration) *Configuration {
	if cfg == nil {
		return nil
	}

	clone := *cfg

	if clone.Database.Password != "" {
		clone.Database.Password = "***"
	}
	if clone.Auth.JWTSecret != "" {
		clone.Auth.JWTSecret = "***"
	}

	return &clone
}

// MaskURL parses a connection URI and replaces the password component with ***.
func MaskURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "***" // Fallback: never leak raw unparseable string
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(username, "***")
		}
	}

	return strings.ReplaceAll(parsed.String(), "%2A%2A%2A", "***")
}

// SecretString automatically masks its content when formatted or printed.
type SecretString string

func (s SecretString) String() string {
	if s == "" {
		return ""
	}
	return "***"
}

func (s SecretString) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`""`), nil
	}
	return []byte(`"***"`), nil
}

// Expose returns the raw underlying sensitive string only when explicitly called.
func (s SecretString) Expose() string {
	return string(s)
}

func ExampleLog(cfg *Configuration) {
	safe := Redacted(cfg)
	fmt.Printf("Safe to log: Host=%s, User=%s, Pass=%s\n",
		safe.Database.Host, safe.Database.User, safe.Database.Password)
}
