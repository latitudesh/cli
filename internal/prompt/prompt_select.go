package prompt

import (
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
)

type InputSelect struct {
	Name  string
	Label string
	Items []string
}

func NewInputSelect(name string, label string, items []string) *InputSelect {
	return &InputSelect{
		Name:  name,
		Label: label,
		Items: items,
	}
}

func (p *InputSelect) AssignValue(attributes interface{}) {
	currentValue := utils.GetFieldValue(attributes, p.Name).String()

	if currentValue == "" {
		value, err := tui.RunList(p.Label, p.Items, nil)
		if err != nil {
			return
		}

		if value == "SKIP" {
			return
		}

		utils.AssignValue(attributes, p.Name, value)
	}
}
