package utils

import (
	"testing"
	"time"
)

func TestParseTimeRefRelative(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		in   string
		want time.Time
	}{
		{"30m", now.Add(-30 * time.Minute)},
		{"24h", now.Add(-24 * time.Hour)},
		{"7d", now.Add(-7 * 24 * time.Hour)},
		{"2w", now.Add(-14 * 24 * time.Hour)},
		{"90s", now.Add(-90 * time.Second)},
	}

	for _, c := range cases {
		got, err := ParseTimeRef(c.in, now)
		if err != nil {
			t.Errorf("ParseTimeRef(%q) returned error: %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseTimeRef(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseTimeRefAbsolute(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-06-01", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-06-01T15:04:05", time.Date(2026, 6, 1, 15, 4, 5, 0, time.UTC)},
		{"2026-06-01T15:04:05Z", time.Date(2026, 6, 1, 15, 4, 5, 0, time.UTC)},
	}

	for _, c := range cases {
		got, err := ParseTimeRef(c.in, now)
		if err != nil {
			t.Errorf("ParseTimeRef(%q) returned error: %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseTimeRef(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseTimeRefInvalid(t *testing.T) {
	now := time.Now()

	for _, in := range []string{"", "yesterday", "24x", "h24", "2026-13-45"} {
		if _, err := ParseTimeRef(in, now); err == nil {
			t.Errorf("ParseTimeRef(%q) expected error, got nil", in)
		}
	}
}

func TestFormatISO8601(t *testing.T) {
	loc := time.FixedZone("BRT", -3*60*60)
	in := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)

	if got, want := FormatISO8601(in), "2026-06-01T12:00:00"; got != want {
		t.Errorf("FormatISO8601 = %q, want %q", got, want)
	}
}
