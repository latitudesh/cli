package renderer

import (
	"io"
	"os"
	"strings"
	"testing"

	outputTable "github.com/latitudesh/lsh/internal/output/table"
	"github.com/spf13/viper"
)

// sampleRow is a minimal ResponseData used to drive the structured renderers.
type sampleRow struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

func (s sampleRow) TableRow() outputTable.Row {
	return outputTable.Row{
		"id":     {Value: s.ID, Label: "ID"},
		"name":   {Value: s.Name, Label: "Name"},
		"status": {Value: s.Status, Label: "Status"},
	}
}

func sampleData() []ResponseData {
	return []ResponseData{
		sampleRow{ID: "srv_1", Name: "alpha", Status: "on"},
		sampleRow{ID: "srv_2", Name: "beta", Status: "off"},
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func resetOutputViper() {
	viper.Set("output", "")
	viper.Set("output_explicit", false)
	viper.Set("json", false)
	viper.Set("query", "")
}

func TestResolveFormatPrecedence(t *testing.T) {
	defer resetOutputViper()

	cases := []struct {
		name     string
		output   string
		explicit bool
		json     bool
		want     Format
	}{
		{"default", "", false, false, FormatTable},
		{"output json (explicit)", "json", true, false, FormatJSON},
		{"output yaml (explicit)", "yaml", true, false, FormatYAML},
		{"output csv (explicit)", "csv", true, false, FormatCSV},
		{"json shortcut", "", false, true, FormatJSON},
		{"explicit -o yaml beats --json", "yaml", true, true, FormatYAML},
		// Regression for greptile P2: an explicit -o table must beat --json.
		{"explicit -o table beats --json", "table", true, true, FormatTable},
		// Without "explicit", a default "table" reading must NOT block --json.
		{"default table + --json yields JSON", "table", false, true, FormatJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetOutputViper()
			viper.Set("output", tc.output)
			viper.Set("output_explicit", tc.explicit)
			viper.Set("json", tc.json)
			if got := ResolveFormat(); got != tc.want {
				t.Fatalf("ResolveFormat() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateOutputSelection(t *testing.T) {
	defer resetOutputViper()

	resetOutputViper()
	viper.Set("output", "bogus")
	if err := ValidateOutputSelection(); err == nil {
		t.Fatal("expected error for invalid --output")
	}

	resetOutputViper()
	viper.Set("query", "[].id") // table format
	if err := ValidateOutputSelection(); err == nil {
		t.Fatal("expected error for --query with table format")
	}

	resetOutputViper()
	viper.Set("output", "json")
	viper.Set("query", "[].id")
	if err := ValidateOutputSelection(); err != nil {
		t.Fatalf("unexpected error for --query with json: %v", err)
	}

	// L3: --output is validated even when --json is set.
	resetOutputViper()
	viper.Set("json", true)
	viper.Set("output", "bogus")
	if err := ValidateOutputSelection(); err == nil {
		t.Fatal("expected error for invalid --output even with --json")
	}

	// M1: a malformed JMESPath fails fast at validation time.
	resetOutputViper()
	viper.Set("output", "json")
	viper.Set("query", "[invalid(")
	if err := ValidateOutputSelection(); err == nil {
		t.Fatal("expected error for malformed --query expression")
	}

	// A valid JMESPath passes.
	resetOutputViper()
	viper.Set("output", "json")
	viper.Set("query", "[?status=='on'].id")
	if err := ValidateOutputSelection(); err != nil {
		t.Fatalf("unexpected error for valid --query: %v", err)
	}
}

func TestRenderJSON(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()

	out := captureStdout(t, func() { JSONRenderer{}.Render(sampleData()) })
	if !strings.Contains(out, `"id": "srv_1"`) || !strings.Contains(out, `"name": "beta"`) {
		t.Fatalf("unexpected JSON output:\n%s", out)
	}
}

func TestRenderJSONEmptyIsArray(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()

	out := strings.TrimSpace(captureStdout(t, func() { JSONRenderer{}.Render(nil) }))
	if out != "[]" {
		t.Fatalf("expected empty JSON array, got %q", out)
	}
}

func TestRenderYAML(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()

	out := captureStdout(t, func() { YAMLRenderer{}.Render(sampleData()) })
	// "off"/"on" are YAML boolean keywords, so a correct encoder quotes them.
	if !strings.Contains(out, "id: srv_1") || !strings.Contains(out, `status: "off"`) {
		t.Fatalf("unexpected YAML output:\n%s", out)
	}
}

func TestRenderCSV(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()

	out := captureStdout(t, func() { CSVRenderer{}.Render(sampleData()) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d lines:\n%s", len(lines), out)
	}
	// id sorts first per the preferred column order.
	if !strings.HasPrefix(lines[0], "id,name,status") {
		t.Fatalf("unexpected CSV header: %q", lines[0])
	}
	if !strings.Contains(lines[1], "srv_1") {
		t.Fatalf("unexpected CSV first row: %q", lines[1])
	}
}

func TestQueryProjectsIDs(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()
	// Equivalent to the PD-6072 acceptance example: only on servers' ids.
	viper.Set("query", "[?status=='on'].id")

	out := strings.TrimSpace(captureStdout(t, func() { JSONRenderer{}.Render(sampleData()) }))
	if !strings.Contains(out, "srv_1") || strings.Contains(out, "srv_2") {
		t.Fatalf("query should return only srv_1, got:\n%s", out)
	}
}

func TestQueryToCSVScalarList(t *testing.T) {
	defer resetOutputViper()
	resetOutputViper()
	viper.Set("query", "[].id")

	out := strings.TrimSpace(captureStdout(t, func() { CSVRenderer{}.Render(sampleData()) }))
	lines := strings.Split(out, "\n")
	if lines[0] != "value" {
		t.Fatalf("expected scalar CSV column 'value', got %q", lines[0])
	}
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 values, got %d:\n%s", len(lines), out)
	}
}
