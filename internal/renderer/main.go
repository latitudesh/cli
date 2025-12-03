package renderer

import (
	"os"

	outputTable "github.com/latitudesh/lsh/internal/output/table"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

type ResponseData interface {
	TableRow() outputTable.Row
}

type Renderer interface {
	Render(data []ResponseData)
}

// GetRenderer returns the appropriate renderer
func GetRenderer() Renderer {
	// Check if should use classic output (for scripts/CI)
	if os.Getenv("LSH_CLASSIC_OUTPUT") == "true" {
		return TableRenderer{} // Old ASCII
	}

	// Check if JSON was requested via --json flag or -o json
	if viper.GetBool("json") || viper.GetString("output") == "json" {
		return JSONRenderer{}
	}

	// If stdout is not a terminal (e.g., pipe), use table output
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return TableRenderer{}
	}

	// Default: use interactive Bubble Tea
	return BubbleTeaRenderer{}
}

// Render renders the data using the appropriate renderer
func Render(data []ResponseData) {
	renderer := GetRenderer()
	renderer.Render(data)
}
