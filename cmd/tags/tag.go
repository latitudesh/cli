package tags

import (
	"context"

	"github.com/go-openapi/strfmt"
	sdk "github.com/latitudesh/latitudesh-go"
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
	Attributes sdk.Tag
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

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(attr.ID),
		},
		"name": table.Cell{
			Label: "Name",
			Value: table.String(attr.Name),
		},
		"description": table.Cell{
			Label: "Description",
			Value: table.String(attr.Description),
		},
		"slug": table.Cell{
			Label: "Slug",
			Value: table.String(attr.Slug),
		},
		"team": table.Cell{
			Label: "Team",
			Value: table.String(attr.TeamName),
		},
		"color": table.Cell{
			Label: "Color",
			Value: table.String(attr.Color),
		},
	}
}
