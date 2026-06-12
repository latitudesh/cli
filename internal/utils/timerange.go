package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var relativeTimeRe = regexp.MustCompile(`^(\d+)([smhdw])$`)

// ParseTimeRef resolves a user-supplied point in time. It accepts a
// relative duration looking back from now ("30m", "24h", "7d", "2w")
// or an absolute date/datetime ("2026-06-01", "2026-06-01T15:04:05",
// RFC3339).
func ParseTimeRef(value string, now time.Time) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty time reference")
	}

	if m := relativeTimeRe.FindStringSubmatch(v); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q", value)
		}
		var unit time.Duration
		switch m[2] {
		case "s":
			unit = time.Second
		case "m":
			unit = time.Minute
		case "h":
			unit = time.Hour
		case "d":
			unit = 24 * time.Hour
		case "w":
			unit = 7 * 24 * time.Hour
		}
		return now.Add(-time.Duration(n) * unit), nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time reference %q: use a duration (e.g. 24h, 7d) or an ISO date (e.g. 2026-06-01 or 2026-06-01T15:04:05)", value)
}

// FormatISO8601 renders t the way the Latitude API expects filter
// timestamps: ISO 8601 seconds precision, UTC. The "Z" suffix is
// deliberately omitted — the events endpoint 500s when it is present.
func FormatISO8601(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}
