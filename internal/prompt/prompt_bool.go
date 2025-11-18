package prompt

import (
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
)

type InputBool struct {
	Name  string
	Label string
}

func NewInputBool(name string, label string) *InputBool {
	return &InputBool{
		Name:  name,
		Label: label,
	}
}

func (p *InputBool) AssignValue(attributes interface{}) {
	currentValue := utils.GetFieldValue(attributes, p.Name).Bool()

	if !currentValue {
		value, err := tui.RunConfirm(p.Label)
		if err != nil {
			return
		}

		utils.AssignValue(attributes, p.Name, value)
	}
}
