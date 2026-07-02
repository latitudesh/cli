package cli

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func strPtr(s string) *string { return &s }

func initiators(nqns ...string) []components.Initiators {
	out := make([]components.Initiators, 0, len(nqns))
	for _, n := range nqns {
		n := n
		out = append(out, components.Initiators{Nqn: &n})
	}
	return out
}

func TestAttachedStatus(t *testing.T) {
	const local = "nqn.2014-08.org.nvmexpress:uuid:this-host"

	cases := []struct {
		name       string
		initiators []components.Initiators
		localNQN   string
		want       string
	}{
		{name: "no initiators", initiators: nil, localNQN: local, want: "No"},
		{name: "empty initiators", initiators: initiators(), localNQN: local, want: "No"},
		{name: "attached elsewhere", initiators: initiators("nqn.2014-08.org.nvmexpress:uuid:other"), localNQN: local, want: "Yes"},
		{name: "attached to this host", initiators: initiators("nqn.2014-08.org.nvmexpress:uuid:other", local), localNQN: local, want: "Yes (this host)"},
		{name: "this host match is case-insensitive/trimmed", initiators: initiators("  NQN.2014-08.ORG.NVMEXPRESS:UUID:THIS-HOST "), localNQN: local, want: "Yes (this host)"},
		{name: "no local nqn known", initiators: initiators(local), localNQN: "", want: "Yes"},
		{name: "initiator with nil nqn", initiators: []components.Initiators{{Nqn: nil}}, localNQN: local, want: "Yes"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attachedStatus(tc.initiators, tc.localNQN); got != tc.want {
				t.Fatalf("attachedStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewVolumeRow(t *testing.T) {
	const local = "nqn.2014-08.org.nvmexpress:uuid:this-host"
	size := int64(100)

	vol := components.VolumeData{
		ID: strPtr("vol_abc123"),
		Attributes: &components.VolumeDataAttributes{
			Name:       strPtr("my-volume"),
			SizeInGb:   &size,
			Project:    &components.ProjectInclude{Slug: strPtr("proj-x"), Name: strPtr("Project X")},
			Initiators: initiators(local),
		},
	}

	row := newVolumeRow(vol, local)

	if row.ID != "vol_abc123" {
		t.Errorf("ID = %q, want vol_abc123", row.ID)
	}
	if row.Name != "my-volume" {
		t.Errorf("Name = %q, want my-volume", row.Name)
	}
	if row.SizeInGB != 100 {
		t.Errorf("SizeInGB = %d, want 100", row.SizeInGB)
	}
	if row.Project != "proj-x" { // slug preferred over name
		t.Errorf("Project = %q, want proj-x", row.Project)
	}
	if row.Attached != "Yes (this host)" {
		t.Errorf("Attached = %q, want %q", row.Attached, "Yes (this host)")
	}

	// TableRow exposes the computed attached column.
	tr := row.TableRow()
	if cell, ok := tr["attached"]; !ok || cell.Value != "Yes (this host)" {
		t.Errorf("TableRow attached cell = %+v, want value %q", cell, "Yes (this host)")
	}
}

func TestNewVolumeRowFallsBackToProjectName(t *testing.T) {
	vol := components.VolumeData{
		ID: strPtr("vol_x"),
		Attributes: &components.VolumeDataAttributes{
			Project: &components.ProjectInclude{Name: strPtr("Only Name")},
		},
	}

	row := newVolumeRow(vol, "")
	if row.Project != "Only Name" {
		t.Errorf("Project = %q, want %q", row.Project, "Only Name")
	}
	if row.Attached != "No" {
		t.Errorf("Attached = %q, want No", row.Attached)
	}
}
