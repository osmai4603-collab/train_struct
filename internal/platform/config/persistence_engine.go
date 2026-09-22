package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// =============================================================================
// Atomic Persistence Engine
// =============================================================================

// ConfigFilePerm defines strict POSIX file permissions (Read/Write for current user only).
const ConfigFilePerm = os.FileMode(0o600)

// ConfigDirPerm defines strict directory permissions for configuration storage.
const ConfigDirPerm = os.FileMode(0o700)

// PersistenceEngine provides atomic, crash-resilient file persistence for configuration objects.
type PersistenceEngine struct{}

// NewPersistenceEngine instantiates a persistence engine.
func NewPersistenceEngine() *PersistenceEngine {
	return &PersistenceEngine{}
}

// Save atomically writes the configuration to disk in formatted JSON with strict 0600 permissions.
// To prevent corrupted or partially written files on crashes or power failures, it writes to a
// temporary file in the destination directory, syncs to disk, and performs an atomic POSIX rename.
func (p *PersistenceEngine) Save(targetPath string, cfg *Configuration) error {
	if cfg == nil {
		return fmt.Errorf("cannot persist nil configuration")
	}

	cleanPath := filepath.Clean(targetPath)
	dir := filepath.Dir(cleanPath)

	if err := os.MkdirAll(dir, ConfigDirPerm); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal configuration to json: %w", err)
	}
	// Append final newline
	data = append(data, '\n')

	// Create temporary file in the same directory to guarantee atomic rename across filesystem boundaries
	tempFile, err := os.CreateTemp(dir, fmt.Sprintf(".%s.tmp.*", filepath.Base(cleanPath)))
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup in case of any failure prior to rename
	cleanup := true
	defer func() {
		if cleanup {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// Enforce 0600 permissions
	if err := tempFile.Chmod(ConfigFilePerm); err != nil {
		return fmt.Errorf("chmod config temp file: %w", err)
	}

	// Write content
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("write config temp file: %w", err)
	}

	// Flush OS kernel buffers to non-volatile storage
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync config temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close config temp file: %w", err)
	}

	// Atomic rename replaces the target file instantaneously
	if err := os.Rename(tempPath, cleanPath); err != nil {
		return fmt.Errorf("atomic rename %s to %s: %w", tempPath, cleanPath, err)
	}

	cleanup = false
	return nil
}

// LoadFile reads and decodes JSON configuration data directly from disk.
func (p *PersistenceEngine) LoadFile(path string, target *Configuration) error {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("read config file %s: %w", cleanPath, err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal config json %s: %w", cleanPath, err)
	}

	return nil
}
