package cmdflag

import (
	"fmt"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/spf13/pflag"
)

type StringSlice struct {
	Name         string
	Label        string
	Description  string
	defaultValue []string
	Value        *[]string
	Required     bool
}

func (f *StringSlice) GetValue() interface{} {
	return *f.Value
}

func (f *StringSlice) GetName() string {
	return f.Name
}

func (f *StringSlice) IsRequired() bool {
	return f.Required
}

func (f *StringSlice) Prompt() prompt.PromptInput {
	return prompt.NewInputStringList(f.Name, f.label())
}

func (f *StringSlice) Register(s *pflag.FlagSet) {
	f.Value = s.StringSlice(f.Name, f.defaultValue, f.description())
}

func (f *StringSlice) label() string {
	if f.Required {
		return f.Label
	}

	return fmt.Sprintf("[Optional] %v", f.Label)
}

func (f *StringSlice) description() string {
	if !f.Required {
		return f.Description
	}

	return fmt.Sprintf("[Required] %v", f.Description)
}

func (f *StringSlice) UpdateOptions(options interface{}) {
	//Function needed to implement interface
}
