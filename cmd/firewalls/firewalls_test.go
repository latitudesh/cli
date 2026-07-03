package firewalls

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

const firewallFixture = `{
  "data": {
    "id": "fw_XQvNevboR5zpb",
    "type": "firewalls",
    "attributes": {
      "name": "web",
      "rules": [
        {"from": "ANY", "to": "ANY", "protocol": "TCP", "port": "22", "description": "ssh"},
        {"from": "ANY", "to": "ANY", "protocol": "TCP", "port": "443"}
      ],
      "project": {"id": "proj_x", "slug": "my-project", "name": "My Project"}
    }
  }
}`

// TestFirewallPayloadDecoding pins the SDK's typed firewall envelope to the
// shape the live API returns, so a schema regression fails here instead of
// rendering an empty table.
func TestFirewallPayloadDecoding(t *testing.T) {
	var payload components.Firewall
	if err := json.Unmarshal([]byte(firewallFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal firewall fixture: %v", err)
	}
	if payload.Data == nil {
		t.Fatal("expected firewall data, got nil")
	}

	fw := Firewall{FirewallData: *payload.Data}
	row := fw.TableRow()

	expectations := map[string]string{
		"id":      "fw_XQvNevboR5zpb",
		"name":    "web",
		"project": "my-project",
		"rules":   "2",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestFirewallAssignmentTableRow(t *testing.T) {
	fwID := "fw_x"
	srvID := "sv_x"
	host := "web-01"
	ip := "1.2.3.4"
	asgID := "fwasg_x"

	asg := FirewallAssignment{FirewallAssignmentData: components.FirewallAssignmentData{
		ID: &asgID,
		Attributes: &components.FirewallAssignmentDataAttributes{
			FirewallID: &fwID,
			Server: &components.FirewallAssignmentDataServer{
				ID:          &srvID,
				Hostname:    &host,
				PrimaryIpv4: &ip,
			},
		},
	}}

	row := asg.TableRow()
	expectations := map[string]string{
		"id":           "fwasg_x",
		"firewall_id":  "fw_x",
		"server_id":    "sv_x",
		"hostname":     "web-01",
		"primary_ipv4": "1.2.3.4",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestParseRulesFlagInline(t *testing.T) {
	rules, err := parseRulesFlag(`[{"from":"ANY","to":"ANY","protocol":"TCP","port":"22"}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Port == nil || *rules[0].Port != "22" {
		t.Errorf("port = %v, want 22", rules[0].Port)
	}
	if rules[0].Protocol == nil || *rules[0].Protocol != operations.CreateFirewallProtocolTCP {
		t.Errorf("protocol = %v, want TCP", rules[0].Protocol)
	}
}

func TestParseRulesFlagFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	content := `[{"from":"192.168.1.0/24","to":"ANY","protocol":"UDP","port":"53","description":"dns"}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("could not write rules file: %v", err)
	}

	rules, err := parseRulesFlag("@" + path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].From == nil || *rules[0].From != "192.168.1.0/24" {
		t.Errorf("from = %v, want 192.168.1.0/24", rules[0].From)
	}
	if rules[0].Protocol == nil || *rules[0].Protocol != operations.CreateFirewallProtocolUDP {
		t.Errorf("protocol = %v, want UDP", rules[0].Protocol)
	}
}

func TestParseRulesFlagEmpty(t *testing.T) {
	rules, err := parseRulesFlag("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil rules, got %v", rules)
	}
}

func TestParseRulesFlagMissingFile(t *testing.T) {
	if _, err := parseRulesFlag("@/no/such/rules.json"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestParseRulesFlagInvalidJSON(t *testing.T) {
	if _, err := parseRulesFlag("not json"); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestBuildCreateFirewallRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(path, []byte(`[{"protocol":"TCP","port":"80"}]`), 0o600); err != nil {
		t.Fatalf("could not write rules file: %v", err)
	}

	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--name", "web", "--project", "my-project", "--rules", "@" + path}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildCreateFirewallRequest(cmd)
	if err != nil {
		t.Fatalf("buildCreateFirewallRequest returned error: %v", err)
	}

	if request.Data.Type != operations.CreateFirewallTypeFirewalls {
		t.Errorf("type = %v, want firewalls", request.Data.Type)
	}
	if request.Data.Attributes.Name != "web" {
		t.Errorf("name = %q, want web", request.Data.Attributes.Name)
	}
	if request.Data.Attributes.Project != "my-project" {
		t.Errorf("project = %q, want my-project", request.Data.Attributes.Project)
	}
	if len(request.Data.Attributes.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(request.Data.Attributes.Rules))
	}
}

func TestBuildCreateFirewallRequestRequiresName(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--project", "my-project"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildCreateFirewallRequest(cmd); err == nil {
		t.Error("expected error when --name missing, got nil")
	}
}

func TestBuildCreateFirewallRequestRequiresProject(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--name", "web"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildCreateFirewallRequest(cmd); err == nil {
		t.Error("expected error when --project missing, got nil")
	}
}

func TestBuildUpdateFirewallRequestRequiresAFlag(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildUpdateFirewallRequest(cmd); err == nil {
		t.Error("expected error when neither --name nor --rules is provided, got nil")
	}
}

func TestBuildUpdateFirewallRequestPartial(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--name", "renamed"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildUpdateFirewallRequest(cmd)
	if err != nil {
		t.Fatalf("buildUpdateFirewallRequest returned error: %v", err)
	}
	if request.Data.Attributes.Name == nil || *request.Data.Attributes.Name != "renamed" {
		t.Errorf("name = %v, want renamed", request.Data.Attributes.Name)
	}
	// Rules were not provided, so they must be left unset.
	if request.Data.Attributes.Rules != nil {
		t.Errorf("expected rules to be nil when --rules not passed, got %v", request.Data.Attributes.Rules)
	}
}

func TestBuildUpdateFirewallRequestRules(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--rules", `[{"protocol":"TCP","port":"22"}]`}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	request, err := buildUpdateFirewallRequest(cmd)
	if err != nil {
		t.Fatalf("buildUpdateFirewallRequest returned error: %v", err)
	}
	if request.Data.Attributes.Name != nil {
		t.Errorf("expected name to be nil, got %v", request.Data.Attributes.Name)
	}
	if len(request.Data.Attributes.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(request.Data.Attributes.Rules))
	}
}

func TestBuildCreateAssignmentRequest(t *testing.T) {
	cmd := NewAssignmentsCreateCmd()
	if err := cmd.Flags().Parse([]string{"--firewall", "fw_x", "--server", "sv_x"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	firewallID, body, err := buildCreateAssignmentRequest(cmd)
	if err != nil {
		t.Fatalf("buildCreateAssignmentRequest returned error: %v", err)
	}
	if firewallID != "fw_x" {
		t.Errorf("firewallID = %q, want fw_x", firewallID)
	}
	if body.Data.Type != operations.CreateFirewallAssignmentFirewallsAssignmentsTypeFirewallAssignments {
		t.Errorf("type = %v, want firewall_assignments", body.Data.Type)
	}
	if body.Data.Attributes == nil || body.Data.Attributes.ServerID != "sv_x" {
		t.Errorf("server_id = %v, want sv_x", body.Data.Attributes)
	}
}

func TestBuildCreateAssignmentRequestRequiresFirewall(t *testing.T) {
	cmd := NewAssignmentsCreateCmd()
	if err := cmd.Flags().Parse([]string{"--server", "sv_x"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, _, err := buildCreateAssignmentRequest(cmd); err == nil {
		t.Error("expected error when --firewall missing, got nil")
	}
}

func TestAssignmentsDeleteRequiresExactlyOneArg(t *testing.T) {
	cmd := NewAssignmentsDeleteCmd()

	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with no args, got nil")
	}
	if err := cmd.Args(cmd, []string{"fwasg_x", "fwasg_y"}); err == nil {
		t.Error("expected error with two args, got nil")
	}
	if err := cmd.Args(cmd, []string{"fwasg_x"}); err != nil {
		t.Errorf("expected no error with one arg, got %v", err)
	}
}

func TestBuildCreateAssignmentRequestRequiresServer(t *testing.T) {
	cmd := NewAssignmentsCreateCmd()
	if err := cmd.Flags().Parse([]string{"--firewall", "fw_x"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, _, err := buildCreateAssignmentRequest(cmd); err == nil {
		t.Error("expected error when --server missing, got nil")
	}
}

// TestBuildUpdateFirewallRequestRejectsEmptyRules guards against a silent
// no-op PATCH when --rules is provided empty.
func TestBuildUpdateFirewallRequestRejectsEmptyRules(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--rules", "  "}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildUpdateFirewallRequest(cmd); err == nil {
		t.Error("expected error for empty --rules, got nil")
	}
}
