package cmdflag

import (
	"fmt"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/spf13/pflag"
)

type Int64 struct {
	Name         string
	Description  string
	Value        *int64
	defaultValue int64
	Required     bool
	Label        string
}

func (f *Int64) GetValue() interface{} {
	return *f.Value
}

func (f *Int64) GetName() string {
	return f.Name
}

func (f *Int64) IsRequired() bool {
	return f.Required
}

func (f *Int64) Prompt() prompt.PromptInput {
	return prompt.NewInputNumber(f.Name, f.label())
}

func (f *Int64) Register(s *pflag.FlagSet) {
	f.Value = s.Int64(f.Name, f.defaultValue, f.description())
}

func (f *Int64) label() string {
	if f.Required {
		return f.Label
	}

	return fmt.Sprintf("[Optional] %v", f.Label)
}

func (f *Int64) description() string {
	if !f.Required {
		return f.Description
	}

	return fmt.Sprintf("[Required] %v", f.Description)
}

func (f *Int64) UpdateOptions(options interface{}) {
	//Function needed to implement interface
}
