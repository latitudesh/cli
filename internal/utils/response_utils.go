package utils

import (
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
)

// Render is a convenient wrapper
func Render(data []renderer.ResponseData) {
	renderer.Render(data)
}

// PrintError prints a formatted error. Recognized API failures are
// rewritten as actionable guidance for the user (e.g. 401 → "run lsh login").
func PrintError(err error) {
	if err == nil {
		return
	}
	msg := HumanizeAPIError(err)
	errorMsg := tui.ErrorStyle.Render("✗ Error: ") + msg
	fmt.Println("\n" + errorMsg + "\n")
}

// HumanizeAPIError rewrites generic SDK error strings into messages
// that tell the user what to do next. Falls back to err.Error() when
// nothing more specific is recognized.
//
// Detection keys off the bracketed "[<status>]" status marker that the
// go-openapi runtime puts on its errors. Two shapes occur in practice:
//
//	[401] Unauthorized  &{...}                       // generic response
//	[POST /servers][401] createServerUnauthorized ...  // typed response
//
// so the marker shows up either at the start of the string or right after
// the "[<method> <path>]" segment. Matching "[401]" only at those two
// anchors (start, or after a "]") catches both shapes while ignoring a bare
// "[401]" that merely appears inside an error payload. Matching loose words
// like "unauthorized" is avoided too — it would capture unrelated validation
// messages such as `"field 'role' has an unauthorized value"`.
func HumanizeAPIError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case hasStatusMarker(s, "401"):
		return "Your authentication token is invalid or revoked.\n" +
			"Run 'lsh login' to sign in again, or see 'lsh help authentication'."
	case hasStatusMarker(s, "403"):
		return "Your token does not have permission for this action.\n" +
			"See 'lsh help authentication' for token scopes."
	}
	return s
}

// hasStatusMarker reports whether s carries a "[<code>]" status marker at one
// of the two anchors the go-openapi runtime emits: the start of the string,
// or immediately after the "]" that closes the "[<method> <path>]" segment.
func hasStatusMarker(s, code string) bool {
	marker := "[" + code + "]"
	return strings.HasPrefix(s, marker) || strings.Contains(s, "]"+marker)
}
