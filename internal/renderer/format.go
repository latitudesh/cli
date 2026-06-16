package renderer

import (
	"fmt"
	"strings"

	"github.com/jmespath/go-jmespath"
	"github.com/spf13/viper"
)

// Format is the output format selected for a command's results. table is the
// human-facing default; json, yaml and csv are the machine-readable formats
// meant for automation, piping and (in the future) an MCP server.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatCSV   Format = "csv"
)

// IsStructured reports whether the format produces machine-readable, queryable
// output. Only structured formats support the --query (JMESPath) flag.
func (f Format) IsStructured() bool {
	return f == FormatJSON || f == FormatYAML || f == FormatCSV
}

// ParseFormat normalizes a user-supplied format string. ok is false for an
// unrecognized value so callers can surface an actionable error.
func ParseFormat(s string) (Format, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "table":
		return FormatTable, true
	case "json":
		return FormatJSON, true
	case "yaml", "yml":
		return FormatYAML, true
	case "csv":
		return FormatCSV, true
	default:
		return "", false
	}
}

// ResolveFormat determines the active output format.
//
// Precedence is flag > env (LSH_OUTPUT) > config > default. The --output flag
// is bound to viper with LSH_OUTPUT as its env source, so viper.GetString
// already applies that chain. The --json shortcut only wins when --output was
// left at its default, mirroring its documented role as "shortcut for
// --output=json".
func ResolveFormat() Format {
	if f, ok := ParseFormat(viper.GetString("output")); ok && f != FormatTable {
		return f
	}
	if viper.GetBool("json") {
		return FormatJSON
	}
	if f, ok := ParseFormat(viper.GetString("output")); ok {
		return f
	}
	return FormatTable
}

// ValidateOutputSelection checks the output-related flags up front so commands
// fail fast with a clear message (and a non-zero exit) instead of silently
// falling back or erroring deep in the render path. It rejects an unknown
// --output value, a --query used with a non-structured format, and a malformed
// JMESPath expression.
func ValidateOutputSelection() error {
	// Validate the format unconditionally — even when --json is set, a typo'd
	// --output should surface early rather than be silently ignored.
	if _, ok := ParseFormat(viper.GetString("output")); !ok {
		return fmt.Errorf("invalid --output %q (valid values: table, json, yaml, csv)", viper.GetString("output"))
	}

	query := strings.TrimSpace(viper.GetString("query"))
	if query == "" {
		return nil
	}
	if !ResolveFormat().IsStructured() {
		return fmt.Errorf("--query requires a structured output format; add -o json, -o yaml, or -o csv")
	}
	// Compile the JMESPath now so an invalid expression fails fast with a
	// non-zero exit code instead of only printing to stderr at render time —
	// scripts and AI agents rely on the exit status.
	if _, err := jmespath.Compile(query); err != nil {
		return fmt.Errorf("invalid --query expression %q: %w", query, err)
	}
	return nil
}
