package virtualmachines

import (
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// VirtualMachines is a renderable collection of virtual machines.
type VirtualMachines struct {
	Data []*VirtualMachine
}

func (m *VirtualMachines) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// VirtualMachine wraps the SDK attributes so it can be rendered as a table row
// and serialized to json/yaml/csv through the shared renderer.
type VirtualMachine struct {
	components.VirtualMachineAttributes
}

func (m *VirtualMachine) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func getStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func (m *VirtualMachine) TableRow() table.Row {
	var (
		name, status, plan, region, ip, os, createdAt string
	)

	if attr := m.Attributes; attr != nil {
		name = getStr(attr.Name)
		if attr.Status != nil {
			status = string(*attr.Status)
		}
		if attr.Plan != nil {
			plan = getStr(attr.Plan.Name)
			if plan == "" {
				plan = getStr(attr.Plan.ID)
			}
		}
		region = getStr(attr.Site)
		ip = getStr(attr.PrimaryIpv4)
		if attr.OperatingSystem != nil {
			os = getStr(attr.OperatingSystem.Slug)
			if os == "" {
				os = getStr(attr.OperatingSystem.Name)
			}
		}
		createdAt = getStr(attr.CreatedAt)
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
		"status": table.Cell{
			Label: "Status",
			Value: table.String(status),
		},
		"plan": table.Cell{
			Label: "Plan",
			Value: table.String(plan),
		},
		"region": table.Cell{
			Label: "Region",
			Value: table.String(region),
		},
		"primary_ipv4": table.Cell{
			Label: "Primary IPv4",
			Value: table.String(ip),
		},
		"operating_system": table.Cell{
			Label: "OS",
			Value: table.String(os),
		},
		"created_at": table.Cell{
			Label: "Created At",
			Value: table.String(createdAt),
		},
	}
}
