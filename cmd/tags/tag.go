package tags

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

type Tags struct {
	Data []*Tag
}

func (m *Tags) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData

	for _, v := range m.Data {
		data = append(data, v)
	}

	return data
}

type Tag struct {
	Attributes components.CustomTagData
}

func (m *Tag) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

// Validate validates this project attributes stats
func (m *Tag) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this project attributes stats based on context it is used
func (m *Tag) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *Tag) MarshalBinary() ([]byte, error) {
	return []byte{}, nil
}

// UnmarshalBinary interface implementation
func (m *Tag) UnmarshalBinary(b []byte) error {
	return nil
}

func (m *Tag) TableRow() table.Row {
	attr := m.Attributes

	// Helper function to safely get string value from pointer
	getStr := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}

	// Get team name if available
	teamName := ""
	if attr.Attributes != nil && attr.Attributes.Team != nil {
		teamName = getStr(attr.Attributes.Team.GetName())
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(attr.ID)),
		},
		"name": table.Cell{
			Label: "Name",
			Value: table.String(getStr(attr.Attributes.GetName())),
		},
		"description": table.Cell{
			Label: "Description",
			Value: table.String(getStr(attr.Attributes.GetDescription())),
		},
		"slug": table.Cell{
			Label: "Slug",
			Value: table.String(getStr(attr.Attributes.GetSlug())),
		},
		"team": table.Cell{
			Label: "Team",
			Value: table.String(teamName),
		},
		"color": table.Cell{
			Label: "Color",
			Value: table.String(getStr(attr.Attributes.GetColor())),
		},
	}
}
