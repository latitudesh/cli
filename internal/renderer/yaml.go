package renderer

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type YAMLRenderer struct{}

func (yr YAMLRenderer) Render(data []ResponseData) {
	if err := renderYAML(data); err != nil {
		reportRenderError(err)
	}
}

func renderYAML(data []ResponseData) error {
	generic, err := structuredData(data)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(generic)
	if err != nil {
		return fmt.Errorf("could not encode the result as YAML: %w", err)
	}

	fmt.Print(string(b))
	return nil
}
