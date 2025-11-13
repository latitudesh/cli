package renderer

import (
	"os"

	outputTable "github.com/latitudesh/lsh/internal/output/table"
	"github.com/spf13/viper"
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

	// Check if JSON was requested
	if viper.GetBool("json") {
		// You can create a JSONRenderer later
		return TableRenderer{} // fallback
	}

	// Default: use interactive Bubble Tea
	return BubbleTeaRenderer{}
}

// Render renders the data using the appropriate renderer
func Render(data []ResponseData) {
	renderer := GetRenderer()
	renderer.Render(data)
}
