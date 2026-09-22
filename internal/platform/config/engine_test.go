package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Unit Tests: Types (Duration & SecretString)
// =============================================================================

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "human string seconds", input: `"5s"`, expected: 5 * time.Second, wantErr: false},
		{name: "human string minutes", input: `"10m"`, expected: 10 * time.Minute, wantErr: false},
		{name: "human string hours", input: `"2h"`, expected: 2 * time.Hour, wantErr: false},
		{name: "numeric integer seconds", input: `15`, expected: 15 * time.Second, wantErr: false},
		{name: "numeric float seconds", input: `2.5`, expected: 2500 * time.Millisecond, wantErr: false},
		{name: "empty string", input: `""`, expected: 0, wantErr: false},
		{name: "null value", input: `null`, expected: 0, wantErr: false},
		{name: "invalid string", input: `"invalid"`, wantErr: true},
		{name: "invalid type boolean", input: `true`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tc.input), &d)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %s, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %s: %v", tc.input, err)
			}
			if d.Duration() != tc.expected {
				t.Fatalf("expected duration %v, got %v", tc.expected, d.Duration())
			}
		})
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	d := Duration(15 * time.Second)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if string(b) != `"15s"` {
		t.Fatalf("expected \"15s\", got %s", string(b))
	}
}

func TestSecretString_Masking(t *testing.T) {
	secret := SecretString("my-super-secret-password")

	// String() must return ***
	if str := secret.String(); str != "***" {
		t.Fatalf("expected masked string '***', got %q", str)
	}

	// MarshalJSON() must serialize to "***"
	b, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if string(b) != `"***"` {
		t.Fatalf("expected '\"***\"', got %s", string(b))
	}

	// Expose() must return the underlying sensitive value
	if raw := secret.Expose(); raw != "my-super-secret-password" {
		t.Fatalf("expected raw secret, got %q", raw)
	}

	// Empty secret handling
	emptySecret := SecretString("")
	if str := emptySecret.String(); str != "" {
		t.Fatalf("expected empty string, got %q", str)
	}
}

// =============================================================================
// Unit Tests: Pipeline Engine & Resolution Layers
// =============================================================================

func TestPipeline_LayerPrecedence(t *testing.T) {
	// Prepare temporary config file (Layer 2)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")
	fileJSON := `{
		"server": {"port": "7000"},
		"database": {"host": "file-db"}
	}`
	if err := os.WriteFile(configPath, []byte(fileJSON), 0o600); err != nil {
		t.Fatalf("failed writing test config file: %v", err)
	}

	// Environment mock (Layer 3)
	envMap := map[string]string{
		"PORT":    "8000",
		"DB_HOST": "env-db",
	}
	envLookup := func(key string) (string, bool) {
		val, ok := envMap[key]
		return val, ok
	}

	// CLI arguments (Layer 4)
	cliArgs := []string{"--port", "9000"}

	// Runtime override (Layer 5)
	override := func(c *Configuration) {
		c.Server.Port = "9999"
	}

	engine := NewEngine(
		WithConfigFile(configPath),
		WithEnvLookup(envLookup),
		WithCLIArgs(cliArgs),
		WithRuntimeOverrides(override),
	)

	cfg, err := engine.Load()
	if err != nil {
		t.Fatalf("engine load failed: %v", err)
	}

	// Layer 5 (Override) should win for Port
	if cfg.Server.Port != "9999" {
		t.Fatalf("expected Port '9999' from Layer 5, got %q", cfg.Server.Port)
	}

	// Layer 3 (Env) should win for DB Host over File and Defaults
	if cfg.Database.Host != "env-db" {
		t.Fatalf("expected DB Host 'env-db' from Layer 3, got %q", cfg.Database.Host)
	}
}

func TestPipeline_EnvAliases(t *testing.T) {
	envMap := map[string]string{
		"HTTP_PORT":         "8888",
		"PGHOST":            "postgres-alias-host",
		"PGPORT":            "5433",
		"PGDATABASE":        "alias_db",
		"PGUSER":            "alias_user",
		"PGPASSWORD":        "alias_pass",
		"HTTP_READ_TIMEOUT": "7s",
	}

	engine := NewEngine(
		WithEnvLookup(func(key string) (string, bool) {
			val, ok := envMap[key]
			return val, ok
		}),
	)

	cfg, err := engine.Load()
	if err != nil {
		t.Fatalf("engine load failed: %v", err)
	}

	if cfg.Server.Port != "8888" {
		t.Errorf("expected port '8888' from HTTP_PORT alias, got %q", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout.Duration() != 7*time.Second {
		t.Errorf("expected read_timeout '7s', got %v", cfg.Server.ReadTimeout.Duration())
	}
	if cfg.Database.Host != "postgres-alias-host" {
		t.Errorf("expected db host from PGHOST alias, got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != "5433" {
		t.Errorf("expected db port '5433' from PGPORT, got %q", cfg.Database.Port)
	}
	if cfg.Database.Name != "alias_db" {
		t.Errorf("expected db name from PGDATABASE, got %q", cfg.Database.Name)
	}
	if cfg.Database.User != "alias_user" {
		t.Errorf("expected db user from PGUSER, got %q", cfg.Database.User)
	}
	if cfg.Database.Password != "alias_pass" {
		t.Errorf("expected db password from PGPASSWORD, got %q", cfg.Database.Password)
	}
}

// =============================================================================
// Unit Tests: Validation Engine & Multi-Error Reporting
// =============================================================================

func TestValidationEngine_FailFastReport(t *testing.T) {
	cfg := DefaultConfiguration()
	cfg.Server.Port = "99999"                           // Out of range port
	cfg.Server.ReadTimeout = Duration(-1 * time.Second) // Non-positive timeout
	cfg.Server.ShutdownTimeout = Duration(2 * time.Second)
	cfg.Server.DrainDuration = Duration(5 * time.Second) // Drain > Shutdown
	cfg.Database.Driver = "invalid_driver"
	cfg.Auth.JWTSecret = "short" // < 32 chars

	validator := NewValidationEngine()
	err := validator.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}

	report, ok := err.(*ValidationReport)
	if !ok {
		t.Fatalf("expected *ValidationReport type, got %T", err)
	}

	// Ensure all errors were accumulated
	errMsg := report.Error()
	expectedSubstrings := []string{
		"server.port",
		"server.read_timeout",
		"server.shutdown_timeout",
		"database.driver",
		"auth.jwt_secret",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(errMsg, sub) {
			t.Errorf("expected diagnostic error report to contain %q, report was:\n%s", sub, errMsg)
		}
	}
}

// =============================================================================
// Unit Tests: Security & Redaction Engine
// =============================================================================

func TestSecurityEngine_Redaction(t *testing.T) {
	cfg := DefaultConfiguration()
	cfg.Database.Password = "super_secret_db_pass"
	cfg.Database.DatabaseURL = "postgres://admin:topsecret@postgres-cluster:5432/train_db?sslmode=require"
	cfg.Auth.JWTSecret = "super-secret-production-jwt-signing-key-long"

	security := NewSecurityEngine()
	redacted := security.Redact(cfg)

	// Original must retain secret credentials
	if cfg.Database.Password != "super_secret_db_pass" {
		t.Errorf("original config password modified unexpectedly")
	}

	// Redacted must mask credentials
	if redacted.Database.Password != "***" {
		t.Errorf("expected password '***', got %q", redacted.Database.Password)
	}
	if redacted.Auth.JWTSecret != "***" {
		t.Errorf("expected jwt secret '***', got %q", redacted.Auth.JWTSecret)
	}

	expectedURL := "postgres://admin:***@postgres-cluster:5432/train_db?sslmode=require"
	if redacted.Database.DatabaseURL != expectedURL {
		t.Errorf("expected masked URL %q, got %q", expectedURL, redacted.Database.DatabaseURL)
	}
}

func TestSecurityEngine_MaskURL_EdgeCases(t *testing.T) {
	security := NewSecurityEngine()

	// Empty URL
	if res := security.MaskURL(""); res != "" {
		t.Errorf("expected empty string for empty url, got %q", res)
	}

	// URL without password
	noPass := "postgres://localhost:5432/db"
	if res := security.MaskURL(noPass); res != noPass {
		t.Errorf("expected unmutated URL %q, got %q", noPass, res)
	}

	// Invalid unparseable URL should not leak plain content
	invalid := "postgres://user:password@%%invalid-url%%"
	if res := security.MaskURL(invalid); res != "***" {
		t.Errorf("expected '***' on parse failure, got %q", res)
	}
}

// =============================================================================
// Unit Tests: Persistence Engine (Atomic Write & Permissions)
// =============================================================================

func TestPersistenceEngine_AtomicSaveAndPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sub", "app_config.json")

	cfg := DefaultConfiguration()
	cfg.Server.Port = "9090"

	persistence := NewPersistenceEngine()
	if err := persistence.Save(targetPath, cfg); err != nil {
		t.Fatalf("failed to atomically save configuration: %v", err)
	}

	// Check file permissions (0600)
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat saved file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected strict file permissions 0600, got %o", perm)
	}

	// Verify content loads back correctly
	var loaded Configuration
	if err := persistence.LoadFile(targetPath, &loaded); err != nil {
		t.Fatalf("failed to load saved configuration: %v", err)
	}

	if loaded.Server.Port != "9090" {
		t.Errorf("expected loaded port '9090', got %q", loaded.Server.Port)
	}
}

// =============================================================================
// Unit Tests: Schema Engine (.env.example Generation)
// =============================================================================

func TestSchemaEngine_IntrospectionAndTemplate(t *testing.T) {
	schema := NewSchemaEngine()
	specs := schema.ExtractSpecs(DefaultConfiguration())

	if len(specs) == 0 {
		t.Fatal("expected extracted specs, got 0")
	}

	foundKeys := make(map[string]bool)
	for _, spec := range specs {
		foundKeys[spec.EnvKey] = true
	}

	requiredKeys := []string{"PORT", "HOST", "READ_TIMEOUT", "DB_DRIVER", "DB_HOST", "JWT_SECRET"}
	for _, req := range requiredKeys {
		if !foundKeys[req] {
			t.Errorf("expected spec for key %q, but not found", req)
		}
	}

	template := schema.GenerateEnvTemplate(specs)
	if !strings.Contains(template, "PORT=") {
		t.Errorf("expected template to contain 'PORT=', got:\n%s", template)
	}
	if !strings.Contains(template, "JWT_SECRET=") {
		t.Errorf("expected template to contain 'JWT_SECRET=', got:\n%s", template)
	}
}

// =============================================================================
// Unit Tests: Full Engine Orchestration
// =============================================================================

func TestEngine_FullOrchestration(t *testing.T) {
	engine := NewEngine(
		WithCLIArgs([]string{"--port", "8085"}),
		WithRuntimeOverrides(func(cfg *Configuration) {
			cfg.Database.Name = "overridden_db"
		}),
	)

	cfg, err := engine.Load()
	if err != nil {
		t.Fatalf("engine load failed: %v", err)
	}

	if cfg.Server.Port != "8085" {
		t.Errorf("expected port '8085', got %q", cfg.Server.Port)
	}
	if cfg.Database.Name != "overridden_db" {
		t.Errorf("expected db name 'overridden_db', got %q", cfg.Database.Name)
	}

	// Validate redacted copy
	redacted := engine.Redact(cfg)
	if redacted.Auth.JWTSecret != "***" {
		t.Errorf("expected redacted JWT secret, got %q", redacted.Auth.JWTSecret)
	}
}

// =============================================================================
// Unit Tests: ByteSize Parsing & Formatting
// =============================================================================

func TestByteSize_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{name: "bytes string", input: `"512B"`, expected: 512, wantErr: false},
		{name: "kilobytes string", input: `"64KB"`, expected: 64 * 1024, wantErr: false},
		{name: "megabytes string", input: `"10MB"`, expected: 10 * 1024 * 1024, wantErr: false},
		{name: "gigabytes string", input: `"2GB"`, expected: 2 * 1024 * 1024 * 1024, wantErr: false},
		{name: "raw numeric bytes", input: `1048576`, expected: 1048576, wantErr: false},
		{name: "empty string", input: `""`, expected: 0, wantErr: false},
		{name: "null value", input: `null`, expected: 0, wantErr: false},
		{name: "invalid unit", input: `"500ZZ"`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sz ByteSize
			err := json.Unmarshal([]byte(tc.input), &sz)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.input, err)
			}
			if sz.Bytes() != tc.expected {
				t.Fatalf("expected %d bytes, got %d", tc.expected, sz.Bytes())
			}
		})
	}
}

// =============================================================================
// Unit Tests: File-Based Secrets (*_FILE)
// =============================================================================

func TestPipeline_FileBasedSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	dbSecretFile := filepath.Join(tmpDir, "db_password.txt")
	jwtSecretFile := filepath.Join(tmpDir, "jwt_secret.txt")

	if err := os.WriteFile(dbSecretFile, []byte("in_file_db_password\n"), 0o600); err != nil {
		t.Fatalf("failed to write db secret file: %v", err)
	}
	if err := os.WriteFile(jwtSecretFile, []byte("in_file_very_secure_jwt_token_key_at_least_32_chars\n"), 0o600); err != nil {
		t.Fatalf("failed to write jwt secret file: %v", err)
	}

	envMap := map[string]string{
		"DB_PASSWORD_FILE": dbSecretFile,
		"JWT_SECRET_FILE":  jwtSecretFile,
	}

	engine := NewEngine(
		WithEnvLookup(func(key string) (string, bool) {
			val, ok := envMap[key]
			return val, ok
		}),
	)

	cfg, err := engine.Load()
	if err != nil {
		t.Fatalf("engine load failed: %v", err)
	}

	if cfg.Database.Password != "in_file_db_password" {
		t.Errorf("expected trimmed db password from file, got %q", cfg.Database.Password)
	}
	if cfg.Auth.JWTSecret != "in_file_very_secure_jwt_token_key_at_least_32_chars" {
		t.Errorf("expected trimmed jwt secret from file, got %q", cfg.Auth.JWTSecret)
	}
}

// =============================================================================
// Unit Tests: Atomic Hot-Reload & Concurrent Read
// =============================================================================

func TestEngine_AtomicReload(t *testing.T) {
	portValue := "8080"
	var mu sync.Mutex

	engine := NewEngine(
		WithRuntimeOverrides(func(cfg *Configuration) {
			mu.Lock()
			cfg.Server.Port = portValue
			mu.Unlock()
		}),
	)

	initial, err := engine.Load()
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	if initial.Server.Port != "8080" {
		t.Fatalf("expected initial port 8080, got %s", initial.Server.Port)
	}

	// Concurrently read while triggering atomic Reload
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					curr := engine.Current()
					if curr == nil || curr.Server.Port == "" {
						t.Errorf("got nil or invalid current config during concurrent read")
					}
				}
			}
		}()
	}

	// Update port dynamically and trigger reload
	mu.Lock()
	portValue = "8090"
	mu.Unlock()

	reloaded, err := engine.Reload()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.Server.Port != "8090" {
		t.Errorf("expected reloaded port 8090, got %s", reloaded.Server.Port)
	}

	close(stop)
	wg.Wait()

	if engine.Current().Server.Port != "8090" {
		t.Errorf("expected Current() to reflect reloaded port 8090, got %s", engine.Current().Server.Port)
	}
}
