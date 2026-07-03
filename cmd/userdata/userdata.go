package userdata

import (
	"encoding/base64"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
)

// getStr aliases utils.Str for brevity in this package.
var getStr = utils.Str

// UserDataList is a renderable collection of user data entries.
type UserDataList struct {
	Data []*UserData
}

func (m *UserDataList) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// UserData wraps the SDK's UserDataProperties so it can be rendered as a table
// row or serialized through the shared renderer. It backs both the
// account-scoped (`lsh user-data`) and project-scoped (`lsh projects user-data`)
// commands, which return the same underlying resource.
type UserData struct {
	components.UserDataProperties
}

func (m *UserData) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

// TableRow renders the compact list columns. The (base64) content is left out
// on purpose — it is exposed decoded in the details view via DetailFields.
func (m *UserData) TableRow() table.Row {
	var description, createdAt, updatedAt string
	if attr := m.Attributes; attr != nil {
		description = getStr(attr.Description)
		createdAt = getStr(attr.CreatedAt)
		updatedAt = getStr(attr.UpdatedAt)
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(m.ID)),
		},
		"description": table.Cell{
			Label: "Description",
			Value: table.String(description),
		},
		"created_at": table.Cell{
			Label: "Created At",
			Value: table.String(createdAt),
		},
		"updated_at": table.Cell{
			Label: "Updated At",
			Value: table.String(updatedAt),
		},
	}
}

// DecodedContent returns the entry's content decoded from base64, falling back
// to the raw value when it is not valid base64.
func (m *UserData) DecodedContent() string {
	if m.Attributes == nil {
		return ""
	}
	content := getStr(m.Attributes.Content)
	if content == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(string(decoded), "\n")
}

// DetailFields exposes the fields shown only in the details view: the project
// association and the decoded cloud-init content.
func (m *UserData) DetailFields() map[string]string {
	fields := make(map[string]string)
	if attr := m.Attributes; attr != nil && attr.Project != nil {
		project := getStr(attr.Project.Slug)
		if project == "" {
			project = getStr(attr.Project.Name)
		}
		if project == "" {
			project = getStr(attr.Project.ID)
		}
		if project != "" {
			fields["Project"] = project
		}
	}
	if content := m.DecodedContent(); content != "" {
		fields["Content"] = content
	}
	return fields
}

// DetailView describes how user data is titled in the interactive views.
func (m *UserData) DetailView() renderer.DetailView {
	return renderer.DetailView{
		Title:        "User Data",
		Noun:         "entries",
		DetailPrefix: "User Data",
		TitleKey:     "ID",
		FieldOrder:   []string{"Project", "Content"},
	}
}
