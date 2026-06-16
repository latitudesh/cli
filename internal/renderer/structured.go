package renderer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmespath/go-jmespath"
	"github.com/spf13/viper"
)

// toGeneric converts the typed rows into plain interface{} structures (maps,
// slices, scalars) by round-tripping through JSON. This is the shape JMESPath
// queries operate on and that the JSON/YAML/CSV encoders consume, so every
// structured format shares one canonical representation of the data.
func toGeneric(data []ResponseData) (interface{}, error) {
	if len(data) == 0 {
		return []interface{}{}, nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// applyQuery runs the --query JMESPath expression over the data when one is
// set. It returns the (possibly transformed) data unchanged when no query is
// present so callers can use it unconditionally.
func applyQuery(data interface{}) (interface{}, error) {
	expr := strings.TrimSpace(viper.GetString("query"))
	if expr == "" {
		return data, nil
	}
	result, err := jmespath.Search(expr, data)
	if err != nil {
		return nil, fmt.Errorf("invalid --query expression %q: %w", expr, err)
	}
	return result, nil
}

// structuredData builds the generic representation and applies --query. It is
// the single entry point the JSON/YAML/CSV renderers use.
func structuredData(data []ResponseData) (interface{}, error) {
	generic, err := toGeneric(data)
	if err != nil {
		return nil, err
	}
	return applyQuery(generic)
}

// reportRenderError surfaces a rendering/query failure on stderr. The Renderer
// interface has no error return (it is called deep inside command flows), so
// this keeps stdout clean for piping while still telling the user what failed.
func reportRenderError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Error: "+err.Error())
}
