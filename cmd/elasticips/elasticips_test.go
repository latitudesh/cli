package elasticips

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const elasticIPFixture = `{
  "data": {
    "id": "eip_XQvNevboR5zpb",
    "type": "elastic_ips",
    "attributes": {
      "address": "203.0.113.10",
      "family": "IPv4",
      "prefix_length": 32,
      "mode": "routed",
      "status": "active",
      "project": {"id": "proj_x", "slug": "my-project", "name": "My Project"},
      "region": {"id": "reg_x", "name": "Chicago"},
      "server": {"id": "sv_x", "hostname": "web-01"}
    }
  }
}`

// TestElasticIPPayloadDecoding pins the SDK's typed Elastic IP envelope to the
// shape the live API returns, so a schema regression fails here instead of
// rendering an empty table.
func TestElasticIPPayloadDecoding(t *testing.T) {
	var payload components.ElasticIP
	if err := json.Unmarshal([]byte(elasticIPFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal elastic IP fixture: %v", err)
	}
	if payload.Data == nil {
		t.Fatal("expected elastic IP data, got nil")
	}

	ip := ElasticIP{ElasticIPData: *payload.Data}
	row := ip.TableRow()

	expectations := map[string]string{
		"id":      "eip_XQvNevboR5zpb",
		"address": "203.0.113.10",
		"family":  "IPv4",
		"status":  "active",
		"project": "my-project",
		"region":  "Chicago",
		"server":  "sv_x",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestBuildCreateRequest(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--project", "my-project", "--server", "sv_x"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildCreateRequest(cmd)
	if err != nil {
		t.Fatalf("buildCreateRequest returned error: %v", err)
	}
	if request.Data.Type != components.CreateElasticIPTypeElasticIps {
		t.Errorf("type = %v, want elastic_ips", request.Data.Type)
	}
	if request.Data.Attributes.ProjectID != "my-project" {
		t.Errorf("project_id = %q, want my-project", request.Data.Attributes.ProjectID)
	}
	if request.Data.Attributes.ServerID != "sv_x" {
		t.Errorf("server_id = %q, want sv_x", request.Data.Attributes.ServerID)
	}
}

func TestBuildCreateRequestRequiresProject(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--server", "sv_x"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildCreateRequest(cmd); err == nil {
		t.Error("expected error when --project missing, got nil")
	}
}

func TestBuildCreateRequestRequiresServer(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--project", "my-project"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildCreateRequest(cmd); err == nil {
		t.Error("expected error when --server missing, got nil")
	}
}

func TestBuildUpdateRequest(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--server", "sv_y"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildUpdateRequest(cmd)
	if err != nil {
		t.Fatalf("buildUpdateRequest returned error: %v", err)
	}
	if request.Data.Attributes.ServerID != "sv_y" {
		t.Errorf("server_id = %q, want sv_y", request.Data.Attributes.ServerID)
	}
}

func TestBuildUpdateRequestRequiresServer(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildUpdateRequest(cmd); err == nil {
		t.Error("expected error when --server missing, got nil")
	}
}

func TestBuildListRequestFilters(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{"--project", "my-project", "--server", "sv_x", "--status", "active"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildListRequest(cmd)
	if err != nil {
		t.Fatalf("buildListRequest returned error: %v", err)
	}
	if request.FilterProject == nil || *request.FilterProject != "my-project" {
		t.Errorf("FilterProject = %v, want my-project", request.FilterProject)
	}
	if request.FilterServer == nil || *request.FilterServer != "sv_x" {
		t.Errorf("FilterServer = %v, want sv_x", request.FilterServer)
	}
	if request.FilterStatus == nil || string(*request.FilterStatus) != "active" {
		t.Errorf("FilterStatus = %v, want active", request.FilterStatus)
	}
}

func TestBuildListRequestNoFilters(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildListRequest(cmd)
	if err != nil {
		t.Fatalf("buildListRequest returned error: %v", err)
	}
	if request.FilterProject != nil || request.FilterServer != nil || request.FilterStatus != nil {
		t.Errorf("expected no filters set by default, got %+v", request)
	}
}

// TestBuildListRequestRejectsInvalidStatus guards the --status enum validation.
func TestBuildListRequestRejectsInvalidStatus(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{"--status", "bogus"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildListRequest(cmd); err == nil {
		t.Error("expected error for invalid --status, got nil")
	}
}
