package storage_filesystems

import (
	"strconv"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// Filesystems is the renderable collection of filesystems.
type Filesystems struct {
	Data []*Filesystem
}

func (m *Filesystems) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// Filesystem wraps the SDK FilesystemData so it can be rendered by the
// shared renderer (table / -o json,yaml / --query).
type Filesystem struct {
	components.FilesystemData
}

func (m *Filesystem) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *Filesystem) TableRow() table.Row {
	fs := m.FilesystemData

	getStr := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}

	var name, size, createdAt, project string
	if attr := fs.Attributes; attr != nil {
		name = getStr(attr.Name)
		if attr.SizeInGb != nil {
			size = strconv.FormatInt(*attr.SizeInGb, 10)
		}
		if attr.CreatedAt != nil {
			createdAt = attr.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
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
			Value: table.String(getStr(fs.ID)),
		},
		"name": table.Cell{
			Label: "Name",
			Value: table.String(name),
		},
		"size_in_gb": table.Cell{
			Label: "Size (GB)",
			Value: table.String(size),
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
