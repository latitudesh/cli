package elasticips

import (
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
)

// getStr aliases utils.Str for brevity in this package.
var getStr = utils.Str

// ElasticIPs is the collection wrapper rendered by `elastic-ips list`.
type ElasticIPs struct {
	Data []*ElasticIP
}

func (m *ElasticIPs) GetData() []renderer.ResponseData {
	data := make([]renderer.ResponseData, 0, len(m.Data))
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// ElasticIP wraps a single SDK ElasticIPData so it can be rendered.
type ElasticIP struct {
	components.ElasticIPData
}

func (m *ElasticIP) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *ElasticIP) TableRow() table.Row {
	var address, status, family, project, region, server string

	if attr := m.Attributes; attr != nil {
		address = getStr(attr.Address)
		if attr.Status != nil {
			status = string(*attr.Status)
		}
		if attr.Family != nil {
			family = string(*attr.Family)
		}
		if attr.Project != nil {
			project = getStr(attr.Project.Slug)
			if project == "" {
				project = getStr(attr.Project.Name)
			}
		}
		if attr.Region != nil {
			region = getStr(attr.Region.Name)
		}
		if attr.Server != nil {
			server = getStr(attr.Server.ID)
		}
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"address": table.Cell{
			Label: "Address",
			Value: table.String(address),
		},
		"family": table.Cell{
			Label: "Family",
			Value: table.String(family),
		},
		"status": table.Cell{
			Label: "Status",
			Value: table.String(status),
		},
		"project": table.Cell{
			Label: "Project",
			Value: table.String(project),
		},
		"region": table.Cell{
			Label: "Region",
			Value: table.String(region),
		},
		"server": table.Cell{
			Label: "Server",
			Value: table.String(server),
		},
	}
}
