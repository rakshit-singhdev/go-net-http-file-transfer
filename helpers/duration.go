package helpers

import "time"

// ParseDuration parses values like "30s", "15m" or "24h".
// It returns fallback when the value is empty, invalid or not positive.
func ParseDuration(value string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fallback
	}

	return d
}
