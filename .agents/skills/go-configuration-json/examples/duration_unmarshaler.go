package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// Resilient Duration Type (Supports Strings "5s" and Numeric Seconds)
// =============================================================================

// Duration wraps time.Duration to provide flexible JSON unmarshaling.
type Duration time.Duration

// Duration returns the standard time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON formats duration as a standard Go duration string (e.g. "5s").
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON parses both human strings ("5s", "10m") and raw numbers (seconds).
func (d *Duration) UnmarshalJSON(b []byte) error {
	var raw interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	switch val := raw.(type) {
	case string:
		if val == "" {
			*d = Duration(0)
			return nil
		}
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", val, err)
		}
		*d = Duration(parsed)
		return nil

	case float64:
		*d = Duration(time.Duration(val * float64(time.Second)))
		return nil

	default:
		return errors.New("duration must be either a string (e.g. '5s') or number of seconds")
	}
}
