package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type JSONRenderer struct{}

func (jr JSONRenderer) Render(data []ResponseData) {
	if len(data) == 0 {
		return
	}

	JSONString, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Could not decode the result as JSON.")
		return
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, JSONString, "", "    "); err != nil {
		fmt.Println("JSON format error")
		return
	}

	fmt.Println(prettyJSON.String())
}
