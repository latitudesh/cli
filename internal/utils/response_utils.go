package utils

import (
	"fmt"
	
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
)

// Render is a convenient wrapper
func Render(data []renderer.ResponseData) {
	renderer.Render(data)
}

// PrintError prints a formatted error
func PrintError(err error) {
	if err == nil {
		return
	}
	
	// Use error style from TUI
	errorMsg := tui.ErrorStyle.Render("✗ Error: ") + err.Error()
	fmt.Println("\n" + errorMsg + "\n")
}
