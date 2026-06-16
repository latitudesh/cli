package renderer

import (
	"encoding/json"
	"fmt"
)

type JSONRenderer struct{}

func (jr JSONRenderer) Render(data []ResponseData) {
	if err := renderJSON(data); err != nil {
		reportRenderError(err)
	}
}

func renderJSON(data []ResponseData) error {
	generic, err := structuredData(data)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(generic, "", "    ")
	if err != nil {
		return fmt.Errorf("could not encode the result as JSON: %w", err)
	}

	fmt.Println(string(b))
	return nil
}
