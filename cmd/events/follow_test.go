package events

import (
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestFollowCursor(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

	t.Run("no --since seeds from now", func(t *testing.T) {
		got := followCursor(&operations.GetEventsRequest{}, now)
		if !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})

	t.Run("--since seeds from the gte instant", func(t *testing.T) {
		gte := "2026-06-17T08:30:00" // FormatISO8601 output
		got := followCursor(&operations.GetEventsRequest{FilterCreatedAtGte: &gte}, now)
		want := time.Date(2026, 6, 17, 8, 30, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if got.Location() != time.UTC {
			t.Errorf("cursor not in UTC: %v", got.Location())
		}
	})

	t.Run("unparseable gte falls back to now", func(t *testing.T) {
		bad := "nonsense"
		got := followCursor(&operations.GetEventsRequest{FilterCreatedAtGte: &bad}, now)
		if !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})
}

func TestEventTime(t *testing.T) {
	str := func(s string) *string { return &s }

	cases := []struct {
		name    string
		in      *components.EventData
		wantOK  bool
		wantUTC string // expected RFC3339-ish, second precision, UTC
	}{
		{
			name:   "nil attributes",
			in:     &components.EventData{},
			wantOK: false,
		},
		{
			name:   "nil created_at",
			in:     &components.EventData{Attributes: &components.EventDataAttributes{}},
			wantOK: false,
		},
		{
			name:    "rfc3339 with zone",
			in:      &components.EventData{Attributes: &components.EventDataAttributes{CreatedAt: str("2026-06-18T15:04:05-03:00")}},
			wantOK:  true,
			wantUTC: "2026-06-18T18:04:05Z",
		},
		{
			name:    "no zone (API filter format)",
			in:      &components.EventData{Attributes: &components.EventDataAttributes{CreatedAt: str("2026-06-18T18:04:05")}},
			wantOK:  true,
			wantUTC: "2026-06-18T18:04:05Z",
		},
		{
			name:   "unparseable",
			in:     &components.EventData{Attributes: &components.EventDataAttributes{CreatedAt: str("nonsense")}},
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := eventTime(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if got.Location() != time.UTC {
				t.Errorf("time not in UTC: %v", got.Location())
			}
			if g := got.Format(time.RFC3339); g != c.wantUTC {
				t.Errorf("got %s, want %s", g, c.wantUTC)
			}
		})
	}
}

func TestFollowCursorRFC3339(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	// A timezone-bearing gte (forward-compat with a future FormatISO8601) must
	// still parse instead of silently falling back to now.
	gte := "2026-06-17T08:30:00-03:00"
	got := followCursor(&operations.GetEventsRequest{FilterCreatedAtGte: &gte}, now)
	want := time.Date(2026, 6, 17, 11, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDedupKey(t *testing.T) {
	str := func(s string) *string { return &s }

	t.Run("prefers the event ID", func(t *testing.T) {
		ev := &components.EventData{ID: str("evt_123")}
		if got := dedupKey(ev); got != "evt_123" {
			t.Errorf("got %q, want evt_123", got)
		}
	})

	t.Run("synthetic key when ID missing", func(t *testing.T) {
		ev := &components.EventData{
			Attributes: &components.EventDataAttributes{
				Action:    str("servers.update"),
				CreatedAt: str("2026-06-18T12:00:00"),
				Target:    &components.Target{ID: str("sv_x")},
			},
		}
		k1 := dedupKey(ev)
		k2 := dedupKey(ev)
		if k1 != k2 {
			t.Errorf("synthetic key not stable: %q vs %q", k1, k2)
		}
		if k1 == "" {
			t.Error("synthetic key should not be empty")
		}
	})

	t.Run("distinct events get distinct keys", func(t *testing.T) {
		a := &components.EventData{Attributes: &components.EventDataAttributes{
			Action: str("servers.update"), CreatedAt: str("2026-06-18T12:00:00"),
		}}
		b := &components.EventData{Attributes: &components.EventDataAttributes{
			Action: str("servers.create"), CreatedAt: str("2026-06-18T12:00:00"),
		}}
		if dedupKey(a) == dedupKey(b) {
			t.Error("expected distinct keys for distinct events")
		}
	})
}

func TestEventDeduper(t *testing.T) {
	str := func(s string) *string { return &s }
	at := func(id, ts string) components.EventData {
		return components.EventData{ID: str(id), Attributes: &components.EventDataAttributes{
			Action: str("servers.update"), CreatedAt: str(ts),
		}}
	}
	ids := func(evs []*components.EventData) []string {
		out := make([]string, 0, len(evs))
		for _, e := range evs {
			if e.ID != nil {
				out = append(out, *e.ID)
			} else {
				out = append(out, "<nil>")
			}
		}
		return out
	}

	t.Run("emits oldest-first and de-dupes the boundary second across polls", func(t *testing.T) {
		d := newEventDeduper(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
		// API returns newest-first.
		page1 := []components.EventData{at("evt_b", "2026-06-18T12:00:05"), at("evt_a", "2026-06-18T12:00:01")}
		got := ids(d.next(page1))
		if len(got) != 2 || got[0] != "evt_a" || got[1] != "evt_b" {
			t.Fatalf("poll1 = %v, want [evt_a evt_b]", got)
		}
		// Poll 2: inclusive gte re-returns evt_b (boundary second) plus a new one.
		page2 := []components.EventData{at("evt_c", "2026-06-18T12:00:05"), at("evt_b", "2026-06-18T12:00:05")}
		got = ids(d.next(page2))
		if len(got) != 1 || got[0] != "evt_c" {
			t.Fatalf("poll2 = %v, want [evt_c] (evt_b already shown)", got)
		}
	})

	t.Run("empty page does not drop boundary dedup state (P1 regression)", func(t *testing.T) {
		d := newEventDeduper(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
		if got := ids(d.next([]components.EventData{at("evt_a", "2026-06-18T12:00:05")})); len(got) != 1 || got[0] != "evt_a" {
			t.Fatalf("poll1 = %v, want [evt_a]", got)
		}
		// A transient empty page must not wipe the boundary-second set.
		if got := ids(d.next(nil)); len(got) != 0 {
			t.Fatalf("poll2 (empty) = %v, want []", got)
		}
		// evt_a comes back (cursor unchanged, gte inclusive) and must not repeat.
		if got := ids(d.next([]components.EventData{at("evt_a", "2026-06-18T12:00:05")})); len(got) != 0 {
			t.Fatalf("poll3 = %v, want [] (evt_a already shown)", got)
		}
	})

	t.Run("undated event is emitted exactly once (P1 regression)", func(t *testing.T) {
		d := newEventDeduper(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
		undated := components.EventData{ID: str("evt_x")} // nil Attributes → unparseable time
		page := []components.EventData{undated}

		if got := ids(d.next(page)); len(got) != 1 || got[0] != "evt_x" {
			t.Fatalf("poll1 = %v, want [evt_x]", got)
		}
		// Same event returned again on the next poll must NOT be reprinted.
		if got := ids(d.next([]components.EventData{undated})); len(got) != 0 {
			t.Fatalf("poll2 = %v, want [] (undated event must not repeat)", got)
		}
	})
}
