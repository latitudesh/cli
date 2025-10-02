package cmdflag

import (
	"fmt"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/spf13/pflag"
)

type String struct {
	Name           string
	CustomFlagName string
	Label          string
	Description    string
	defaultValue   string
	Value          *string
	Options        []string
	Required       bool
}

func (f *String) GetValue() interface{} {
	return *f.Value
}

func (f *String) GetName() string {
	return f.Name
}

func (f *String) IsRequired() bool {
	return f.Required
}

func (f *String) Prompt() prompt.PromptInput {
	if len(f.Options) > 0 {
		return prompt.NewInputSelect(f.Name, f.Label, f.Options)
	}

	return prompt.NewInputText(f.Name, f.label())
}

func (f *String) Register(s *pflag.FlagSet) {
	f.Value = s.String(f.Name, f.defaultValue, f.description())
}

func (f *String) label() string {
	if f.Required {
		return f.Label
	}

	return fmt.Sprintf("[Optional] %v", f.Label)
}

func (f *String) description() string {
	if !f.Required {
		return f.Description
	}

	return fmt.Sprintf("[Required] %v", f.Description)
}

func (f *String) UpdateOptions(options interface{}) {
	option := options.([]string)
	f.Options = append(f.Options, option...)
}
