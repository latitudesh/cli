// Package servers holds the SDK-backed server subcommands added for PD-6075:
// power actions, rescue mode and lock/unlock
// access. Each command talks to the API through lsh.NewClient() and renders
// through the shared renderer so -o json/yaml and --query work everywhere.
package servers

import (
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// getStr safely dereferences an optional string pointer.
func getStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// ServerActionModel renders the response of a server power action.
type ServerActionModel struct {
	ServerID string
	Action   string
	Data     *components.ServerActionData
}

func (m *ServerActionModel) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *ServerActionModel) TableRow() table.Row {
	var id, status string
	if m.Data != nil {
		id = getStr(m.Data.ID)
		if m.Data.Attributes != nil {
			status = getStr(m.Data.Attributes.Status)
		}
	}
	return table.Row{
		"server_id": table.Cell{Label: "Server ID", Value: table.String(m.ServerID)},
		"action":    table.Cell{Label: "Action", Value: table.String(m.Action)},
		"id":        table.Cell{Label: "Action ID", Value: table.String(id)},
		"status":    table.Cell{Label: "Status", Value: table.String(status)},
	}
}

// SimpleServerModel renders a one-line outcome for state changes (lock/unlock,
// rescue mode) that return no body of their own.
type SimpleServerModel struct {
	ServerID string
	State    string
}

func (m *SimpleServerModel) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *SimpleServerModel) TableRow() table.Row {
	return table.Row{
		"server_id": table.Cell{Label: "Server ID", Value: table.String(m.ServerID)},
		"state":     table.Cell{Label: "State", Value: table.String(m.State)},
	}
}
