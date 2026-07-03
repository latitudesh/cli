package output

import (
	"encoding/json"
)

// RenderAsJSON marshals and renders any data structure as pretty JSON
func RenderAsJSON(data interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	return RenderJSON(jsonBytes)
}

