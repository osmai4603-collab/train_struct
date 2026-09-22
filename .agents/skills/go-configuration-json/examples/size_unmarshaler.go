package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// =============================================================================
// Resilient ByteSize Type (Supports "512MB", "1GB", and Raw Bytes)
// =============================================================================

// ByteSize represents a quantity of data in bytes with flexible JSON unmarshaling.
type ByteSize int64

const (
	Byte     ByteSize = 1
	Kilobyte          = 1024 * Byte
	Megabyte          = 1024 * Kilobyte
	Gigabyte          = 1024 * Megabyte
	Terabyte          = 1024 * Gigabyte
)

// Bytes returns the size in raw integer bytes.
func (b ByteSize) Bytes() int64 {
	return int64(b)
}

func (b ByteSize) String() string {
	switch {
	case b >= Terabyte && b%Terabyte == 0:
		return fmt.Sprintf("%dTB", b/Terabyte)
	case b >= Gigabyte && b%Gigabyte == 0:
		return fmt.Sprintf("%dGB", b/Gigabyte)
	case b >= Megabyte && b%Megabyte == 0:
		return fmt.Sprintf("%dMB", b/Megabyte)
	case b >= Kilobyte && b%Kilobyte == 0:
		return fmt.Sprintf("%dKB", b/Kilobyte)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// MarshalJSON formats size as a human-readable string (e.g. "512MB").
func (b ByteSize) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

// UnmarshalJSON parses both human size strings ("512MB", "1GB") and raw numeric byte counts.
func (b *ByteSize) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch val := v.(type) {
	case float64:
		*b = ByteSize(val)
		return nil
	case string:
		parsed, err := ParseByteSize(val)
		if err != nil {
			return err
		}
		*b = parsed
		return nil
	default:
		return errors.New("size must be either a string (e.g. '512MB') or raw numeric bytes")
	}
}

// ParseByteSize parses a human-readable size string into ByteSize.
func ParseByteSize(s string) (ByteSize, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Separate number part from unit part
	i := 0
	for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
		i++
	}

	numStr := strings.TrimSpace(s[:i])
	unitStr := strings.ToUpper(strings.TrimSpace(s[i:]))

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric portion %q: %w", numStr, err)
	}

	var multiplier ByteSize
	switch unitStr {
	case "", "B", "BYTES":
		multiplier = Byte
	case "K", "KB", "KIB":
		multiplier = Kilobyte
	case "M", "MB", "MIB":
		multiplier = Megabyte
	case "G", "GB", "GIB":
		multiplier = Gigabyte
	case "T", "TB", "TIB":
		multiplier = Terabyte
	default:
		return 0, fmt.Errorf("unknown size unit %q (supported: B, KB, MB, GB, TB)", unitStr)
	}

	return ByteSize(num * float64(multiplier)), nil
}
