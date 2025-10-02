package cmdflag

import (
	"fmt"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/spf13/pflag"
)

type Bool struct {
	Name         string
	Label        string
	Description  string
	Value        *bool
	defaultValue bool
	Required     bool
}

func (f *Bool) GetValue() interface{} {
	return *f.Value
}

func (f *Bool) GetName() string {
	return f.Name
}

func (f *Bool) IsRequired() bool {
	return f.Required
}

func (f *Bool) Prompt() prompt.PromptInput {
	return prompt.NewInputBool(f.Name, f.label())
}

func (f *Bool) Register(s *pflag.FlagSet) {
	f.Value = s.Bool(f.Name, f.defaultValue, f.description())
}

func (f *Bool) label() string {
	if f.Required {
		return f.Label
	}

	return fmt.Sprintf("[Optional] %v", f.Label)
}

func (f *Bool) description() string {
	if !f.Required {
		return f.Description
	}

	return fmt.Sprintf("[Required] %v", f.Description)
}

func (f *Bool) UpdateOptions(options interface{}) {
	//Function needed to implement interface
}
