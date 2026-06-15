package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const eventsPageFixture = `{
  "data": [
    {
      "id": "evt_XQvNevboR5zpb",
      "type": "events",
      "attributes": {
        "created_at": "2024-11-12T17:33:24+00:00",
        "action": "virtual_networks.destroy",
        "target": {"id": "vlan_7pWRawkbearD6", "name": "virtual_networks"},
        "project": {"id": null, "name": null, "slug": null},
        "team": {"id": "team_x", "name": "Latitude.sh Labs"},
        "author": {"id": "user_x", "name": "Jane Doe", "email": "jane@example.com"}
      }
    },
    {
      "id": "evt_second",
      "type": "events",
      "attributes": {
        "created_at": "2024-11-12T17:00:00+00:00",
        "action": "servers.create",
        "target": {"id": "sv_x", "name": "servers"},
        "author": {"id": "user_y", "name": "John Doe", "email": "john@example.com"}
      }
    }
  ],
  "meta": {}
}`

// TestEventsPayloadDecoding pins the SDK's typed events envelope to the
// shape the live API actually returns, so a schema regression in a future
// SDK release fails here instead of rendering empty tables.
func TestEventsPayloadDecoding(t *testing.T) {
	var payload components.Events
	if err := json.Unmarshal([]byte(eventsPageFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal events fixture: %v", err)
	}
	data := payload.Data

	if len(data) != 2 {
		t.Fatalf("expected 2 events, got %d", len(data))
	}

	first := Event{EventData: data[0]}
	row := first.TableRow()

	expectations := map[string]string{
		"id":         "evt_XQvNevboR5zpb",
		"action":     "virtual_networks.destroy",
		"target":     "virtual_networks",
		"target_id":  "vlan_7pWRawkbearD6",
		"author":     "jane@example.com",
		"project":    "",
		"created_at": "2024-11-12T17:33:24+00:00",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestBuildEventsRequest(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{
		"--author", "user@example.com",
		"--project", "my-project",
		"--target-type", "servers",
		"--target-type", "projects",
		"--target-id", "sv_x",
		"--action", "servers.create",
		"--since", "24h",
		"--until", "2026-06-10",
	}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildEventsRequest(cmd, now)
	if err != nil {
		t.Fatalf("buildEventsRequest returned error: %v", err)
	}

	if request.FilterAuthor == nil || *request.FilterAuthor != "user@example.com" {
		t.Errorf("FilterAuthor = %v, want user@example.com", request.FilterAuthor)
	}
	if request.FilterProject == nil || *request.FilterProject != "my-project" {
		t.Errorf("FilterProject = %v, want my-project", request.FilterProject)
	}
	if len(request.FilterTargetName) != 2 || request.FilterTargetName[0] != "servers" || request.FilterTargetName[1] != "projects" {
		t.Errorf("FilterTargetName = %v, want [servers projects]", request.FilterTargetName)
	}
	if request.FilterTargetID == nil || *request.FilterTargetID != "sv_x" {
		t.Errorf("FilterTargetID = %v, want sv_x", request.FilterTargetID)
	}
	if request.FilterAction == nil || *request.FilterAction != "servers.create" {
		t.Errorf("FilterAction = %v, want servers.create", request.FilterAction)
	}
	if request.FilterCreatedAtGte == nil || *request.FilterCreatedAtGte != "2026-06-09T12:00:00" {
		t.Errorf("FilterCreatedAtGte = %v, want 2026-06-09T12:00:00", request.FilterCreatedAtGte)
	}
	if request.FilterCreatedAtLte == nil || *request.FilterCreatedAtLte != "2026-06-10T00:00:00" {
		t.Errorf("FilterCreatedAtLte = %v, want 2026-06-10T00:00:00", request.FilterCreatedAtLte)
	}
	if request.PageSize == nil || *request.PageSize != eventsPageSize {
		t.Errorf("PageSize = %v, want %d", request.PageSize, eventsPageSize)
	}
}

func TestBuildEventsRequestDefaults(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildEventsRequest(cmd, time.Now())
	if err != nil {
		t.Fatalf("buildEventsRequest returned error: %v", err)
	}

	if request.FilterAuthor != nil || request.FilterProject != nil || request.FilterTargetID != nil ||
		request.FilterAction != nil || request.FilterCreatedAtGte != nil || request.FilterCreatedAtLte != nil ||
		request.FilterTargetName != nil {
		t.Errorf("expected no filters set by default, got %+v", request)
	}
}

func TestBuildEventsRequestInvalidSince(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{"--since", "yesterday"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	if _, err := buildEventsRequest(cmd, time.Now()); err == nil {
		t.Error("expected error for invalid --since, got nil")
	}
}
