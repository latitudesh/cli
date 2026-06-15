package events

import (
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

type Events struct {
	Data []*Event
}

func (m *Events) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData

	for _, v := range m.Data {
		data = append(data, v)
	}

	return data
}

type Event struct {
	components.EventData
}

func (m *Event) TableRow() table.Row {
	getStr := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}

	var action, createdAt, author, project, targetName, targetID string
	if attr := m.Attributes; attr != nil {
		action = getStr(attr.Action)
		createdAt = getStr(attr.CreatedAt)
		if attr.Author != nil {
			author = getStr(attr.Author.Email)
		}
		if attr.Project != nil {
			project = getStr(attr.Project.Slug)
			if project == "" {
				project = getStr(attr.Project.Name)
			}
		}
		if attr.Target != nil {
			targetName = getStr(attr.Target.Name)
			targetID = getStr(attr.Target.ID)
		}
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"action": table.Cell{
			Label: "Action",
			Value: table.String(action),
		},
		"target": table.Cell{
			Label: "Target",
			Value: table.String(targetName),
		},
		"target_id": table.Cell{
			Label: "Target ID",
			Value: table.String(targetID),
		},
		"author": table.Cell{
			Label: "Author",
			Value: table.String(author),
		},
		"project": table.Cell{
			Label: "Project",
			Value: table.String(project),
		},
		"created_at": table.Cell{
			Label: "Created At",
			Value: table.String(createdAt),
		},
	}
}
