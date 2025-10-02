package prompt

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/latitudesh/lsh/internal/utils"
	"github.com/manifoldco/promptui"
)

type InputNumber struct {
	Name  string
	Label string
}

func NewInputNumber(name, label string) *InputNumber {
	return &InputNumber{
		Name:  name,
		Label: label,
	}
}

func (p *InputNumber) AssignValue(attributes interface{}) {
	currentValue := utils.GetFieldValue(attributes, p.Name)

	if currentValue.IsZero() {
		prompt := promptui.Prompt{
			Label: p.Label,
			Validate: func(s string) error {
				if s == "" {
					return nil
				}

				_, err := valueAsNumber(s)
				if err != nil {
					return fmt.Errorf(err.Error())
				}

				return nil
			},
		}

		value, err := prompt.Run()
		if err != nil {
			fmt.Printf("Failed to read input: %v\n", err)
			os.Exit(1)
		}

		if value == "" {
			return
		}

		numberValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			fmt.Println("Invalid input")
		}

		utils.AssignValue(attributes, p.Name, numberValue)
	}
}

func valueAsNumber(value string) (int64, error) {
	numberValue, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		return 0, errors.New("invalid number")
	}

	return numberValue, nil
}
