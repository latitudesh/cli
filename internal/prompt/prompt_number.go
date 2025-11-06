package prompt

import (
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
)

type InputNumber struct {
	Name  string
	Label string
}

func NewInputNumber(name string, label string) *InputNumber {
	return &InputNumber{
		Name:  name,
		Label: label,
	}
}

func (p *InputNumber) AssignValue(attributes interface{}) {
	currentValue := utils.GetFieldValue(attributes, p.Name).Int()

	if currentValue == 0 {
		value, err := tui.RunNumberInput(p.Label, "Enter a number...")
		if err != nil {
			return
		}
		utils.AssignValue(attributes, p.Name, value)
	}
}
