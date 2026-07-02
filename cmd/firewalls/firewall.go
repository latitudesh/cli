package firewalls

import (
	"fmt"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
)

// getStr aliases utils.Str for brevity in this package.
var getStr = utils.Str

// Firewalls is the collection wrapper rendered by `firewalls list`.
type Firewalls struct {
	Data []*Firewall
}

func (m *Firewalls) GetData() []renderer.ResponseData {
	data := make([]renderer.ResponseData, 0, len(m.Data))
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// Firewall wraps a single SDK FirewallData so it can be rendered.
type Firewall struct {
	components.FirewallData
}

func (m *Firewall) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *Firewall) TableRow() table.Row {
	var name, project string
	rulesCount := 0

	if attr := m.Attributes; attr != nil {
		name = getStr(attr.Name)
		rulesCount = len(attr.Rules)
		if attr.Project != nil {
			project = getStr(attr.Project.Slug)
			if project == "" {
				project = getStr(attr.Project.Name)
			}
			if project == "" {
				project = getStr(attr.Project.ID)
			}
		}
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"name": table.Cell{
			Label: "Name",
			Value: table.String(name),
		},
		"project": table.Cell{
			Label: "Project",
			Value: table.String(project),
		},
		"rules": table.Cell{
			Label: "Rules",
			Value: table.String(fmt.Sprintf("%d", rulesCount)),
		},
	}
}

// DetailView describes how firewalls are titled in the interactive views.
func (m *Firewall) DetailView() renderer.DetailView {
	return renderer.DetailView{
		Title:        "Firewalls",
		Noun:         "firewalls",
		DetailPrefix: "Firewall",
		TitleKey:     "Name",
	}
}

// DetailFields expands each firewall rule as its own line in the interactive
// details view (the compact table column only shows the rule count).
func (m *Firewall) DetailFields() map[string]string {
	fields := make(map[string]string)
	if attr := m.Attributes; attr != nil {
		for i := range attr.Rules {
			r := attr.Rules[i]
			line := strings.TrimSpace(fmt.Sprintf("%s %s %s->%s",
				getStr(r.Protocol), getStr(r.Port), getStr(r.From), getStr(r.To)))
			if desc := getStr(r.Description); desc != "" {
				line += "  (" + desc + ")"
			}
			fields[fmt.Sprintf("Rule %d", i+1)] = line
		}
	}
	return fields
}

// FirewallAssignments is the collection wrapper rendered by
// `firewalls assignments list`.
type FirewallAssignments struct {
	Data []*FirewallAssignment
}

func (m *FirewallAssignments) GetData() []renderer.ResponseData {
	data := make([]renderer.ResponseData, 0, len(m.Data))
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// FirewallAssignment wraps a single SDK FirewallAssignmentData.
type FirewallAssignment struct {
	components.FirewallAssignmentData
}

func (m *FirewallAssignment) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *FirewallAssignment) TableRow() table.Row {
	var firewallID, serverID, hostname, primaryIPv4 string

	if attr := m.Attributes; attr != nil {
		firewallID = getStr(attr.FirewallID)
		if attr.Server != nil {
			serverID = getStr(attr.Server.ID)
			hostname = getStr(attr.Server.Hostname)
			primaryIPv4 = getStr(attr.Server.PrimaryIpv4)
		}
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"firewall_id": table.Cell{
			Label: "Firewall ID",
			Value: table.String(firewallID),
		},
		"server_id": table.Cell{
			Label: "Server ID",
			Value: table.String(serverID),
		},
		"hostname": table.Cell{
			Label: "Hostname",
			Value: table.String(hostname),
		},
		"primary_ipv4": table.Cell{
			Label: "Primary IPv4",
			Value: table.String(primaryIPv4),
		},
	}
}

// FirewallServerAssignment wraps the FirewallServer returned by
// `firewalls assignments create`.
type FirewallServerAssignment struct {
	components.FirewallServer
}

func (m *FirewallServerAssignment) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *FirewallServerAssignment) TableRow() table.Row {
	var firewallID, serverID string
	if attr := m.Attributes; attr != nil {
		firewallID = getStr(attr.FirewallID)
		serverID = getStr(attr.ServerID)
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"firewall_id": table.Cell{
			Label: "Firewall ID",
			Value: table.String(firewallID),
		},
		"server_id": table.Cell{
			Label: "Server ID",
			Value: table.String(serverID),
		},
	}
}
