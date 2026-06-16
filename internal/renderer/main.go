package renderer

import (
	"os"

	outputTable "github.com/latitudesh/lsh/internal/output/table"
	"golang.org/x/term"
)

type ResponseData interface {
	TableRow() outputTable.Row
}

type Renderer interface {
	Render(data []ResponseData)
}

// GetRenderer returns the renderer for the active output format.
//
// Structured formats (json/yaml/csv) are honored everywhere, including when
// stdout is piped — that is the whole point of automation output. The table
// format additionally chooses between the classic ASCII writer (for CI /
// LSH_CLASSIC_OUTPUT / non-TTY pipes) and the interactive Bubble Tea view.
func GetRenderer() Renderer {
	switch ResolveFormat() {
	case FormatJSON:
		return JSONRenderer{}
	case FormatYAML:
		return YAMLRenderer{}
	case FormatCSV:
		return CSVRenderer{}
	}

	// Human-facing table path.
	if os.Getenv("LSH_CLASSIC_OUTPUT") == "true" {
		return TableRenderer{} // Old ASCII
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return TableRenderer{} // Plain table for pipes / CI
	}
	return BubbleTeaRenderer{}
}

// Render renders the data using the appropriate renderer
func Render(data []ResponseData) {
	renderer := GetRenderer()
	renderer.Render(data)
}
