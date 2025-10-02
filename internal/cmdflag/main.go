package cmdflag

import (
	"fmt"
	"os"

	"github.com/go-openapi/swag"
	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FlagSchema interface {
	Register(f *pflag.FlagSet)
	UpdateOptions(options interface{})
	GetValue() interface{}
	GetName() string
	Prompt() prompt.PromptInput
	IsRequired() bool
}

type FlagsSchema []FlagSchema

type Flags struct {
	schema            *FlagsSchema
	FlagSet           *pflag.FlagSet
	PromptDescription string
}

func (f *Flags) Register(s *FlagsSchema) {
	f.schema = s

	for _, v := range *s {
		v.Register(f.FlagSet)
	}
}

func (f *Flags) AddFlagOption(flagName string, option []string) {
	for _, flag := range *f.schema {
		if flag.GetName() == flagName {
			flag.UpdateOptions(option)
		}
	}
}

func (f *Flags) GetInputs() []prompt.PromptInput {
	var inputs []prompt.PromptInput

	for _, v := range *f.schema {
		inputs = append(inputs, v.Prompt())
	}

	return inputs
}

func (f *Flags) AssignValues(params interface{}) error {
	for _, v := range *f.schema {
		value := v.GetValue()

		if !swag.IsZero(value) {
			utils.AssignValue(params, v.GetName(), value)
		}
	}

	if f.interactiveModeEnabled() {
		p := prompt.Prompt{
			Description: f.PromptDescription,
			Inputs:      f.GetInputs(),
		}
		p.Run(params)
	}

	return nil
}

func (f *Flags) PreRun(cmd *cobra.Command, args []string) {
	if f.interactiveModeEnabled() {
		return
	}

	for _, v := range *f.schema {
		if v.IsRequired() {
			cmd.MarkFlagRequired(v.GetName())
		}
	}
}

func (f *Flags) interactiveModeEnabled() bool {
	isDisabled, err := f.FlagSet.GetBool("no-input")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return !isDisabled
}
