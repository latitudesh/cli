package prompt

import (
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
)

type InputText struct {
	Name  string
	Label string
}

func NewInputText(name, label string) *InputText {
	return &InputText{
		Name:  name,
		Label: label,
	}
}

func (p *InputText) AssignValue(attributes interface{}) {
	currentValue := utils.GetFieldValue(attributes, p.Name).String()

	if currentValue == "" {
		value, err := tui.RunTextInput(p.Label, "")
		if err != nil {
			return
		}

		utils.AssignValue(attributes, p.Name, value)
	}
}