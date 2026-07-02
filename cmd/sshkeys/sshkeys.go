package sshkeys

import (
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
)

// getStr aliases utils.Str for brevity in this package.
var getStr = utils.Str

// SSHKeys is a renderable collection of SSH keys.
type SSHKeys struct {
	Data []*SSHKey
}

func (m *SSHKeys) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// SSHKey wraps the SDK's SSHKeyData so it can be rendered as a table row or
// serialized through the shared renderer. It backs both the account-scoped
// (`lsh ssh-keys`) and project-scoped (`lsh projects ssh-keys`) commands, which
// return the same underlying resource.
type SSHKey struct {
	components.SSHKeyData
}

func (m *SSHKey) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

// createdBy resolves who created the key: the user's email, falling back to
// the bare ID when the API does not side-load the user's contact fields.
func (m *SSHKey) createdBy() string {
	if m.Attributes == nil || m.Attributes.User == nil {
		return ""
	}
	if email := getStr(m.Attributes.User.Email); email != "" {
		return email
	}
	return getStr(m.Attributes.User.ID)
}

// TableRow renders the compact list columns. The long fields (public key,
// fingerprint, project, tags) live in the details view via DetailFields.
func (m *SSHKey) TableRow() table.Row {
	var name, createdAt string
	if attr := m.Attributes; attr != nil {
		name = getStr(attr.Name)
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
		"created_by": table.Cell{
			Label: "Created By",
			Value: table.String(m.createdBy()),
		},
		"created_at": table.Cell{
			Label: "Created At",
			Value: table.String(createdAt),
		},
	}
}

// DetailFields exposes the fields shown only in the details view.
func (m *SSHKey) DetailFields() map[string]string {
	fields := make(map[string]string)
	attr := m.Attributes
	if attr == nil {
		return fields
	}

	if attr.Project != nil {
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
	if v := getStr(attr.Fingerprint); v != "" {
		fields["Fingerprint"] = v
	}
	if v := getStr(attr.PublicKey); v != "" {
		fields["Public Key"] = v
	}
	if v := getStr(attr.UpdatedAt); v != "" {
		fields["Updated At"] = v
	}
	if len(attr.Tags) > 0 {
		names := make([]string, 0, len(attr.Tags))
		for i := range attr.Tags {
			if n := getStr(attr.Tags[i].Name); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 0 {
			fields["Tags"] = strings.Join(names, ", ")
		}
	}
	return fields
}

// DetailView describes how SSH keys are titled in the interactive views.
func (m *SSHKey) DetailView() renderer.DetailView {
	return renderer.DetailView{
		Title:        "SSH Keys",
		Noun:         "keys",
		DetailPrefix: "SSH Key",
		TitleKey:     "Name",
		FieldOrder:   []string{"Project", "Fingerprint", "Public Key", "Tags", "Updated At"},
	}
}
