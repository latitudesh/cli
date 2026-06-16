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
// Precedence: an explicit --output flag > --json shortcut > env (LSH_OUTPUT) >
// config > default. An explicit --output always wins — including -o table over
// --json — which is why callers record whether the flag was set via the
// "output_explicit" key (viper.GetString alone can't tell an explicit
// `-o table` from the "table" default).
func ResolveFormat() Format {
	out, ok := ParseFormat(viper.GetString("output"))
	if viper.GetBool("output_explicit") && ok {
		return out
	}
	if viper.GetBool("json") {
		return FormatJSON
	}
	if ok {
		return out
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
