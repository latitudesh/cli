package renderer

import (
	"bytes"
	"testing"

	outputTable "github.com/latitudesh/lsh/internal/output/table"
	"github.com/spf13/viper"
)

func TestParseFormatText(t *testing.T) {
	f, ok := ParseFormat("text")
	if !ok || f != FormatText {
		t.Fatalf("ParseFormat(text) = %q, %v", f, ok)
	}
	if !FormatText.IsStructured() {
		t.Fatal("text must be structured so --query works")
	}
}

func TestTextRendererScalar(t *testing.T) {
	defer resetOutputViper()
	viper.Set("output", "text")
	viper.Set("query", "[0].name")

	out := captureStdout(t, func() { TextRenderer{}.Render(sampleData()) })
	if out != "alpha\n" {
		t.Fatalf("scalar: got %q, want %q", out, "alpha\n")
	}
}

func TestTextRendererScalarList(t *testing.T) {
	defer resetOutputViper()
	viper.Set("output", "text")
	viper.Set("query", "[].id")

	out := captureStdout(t, func() { TextRenderer{}.Render(sampleData()) })
	if out != "srv_1\nsrv_2\n" {
		t.Fatalf("scalar list: got %q", out)
	}
}

func TestTextRendererObjectList(t *testing.T) {
	defer resetOutputViper()
	viper.Set("output", "text")

	out := captureStdout(t, func() { TextRenderer{}.Render(sampleData()) })
	// Keys are sorted: id, name, status.
	want := "srv_1\talpha\ton\nsrv_2\tbeta\toff\n"
	if out != want {
		t.Fatalf("object list: got %q, want %q", out, want)
	}
}

func TestTextRendererObject(t *testing.T) {
	defer resetOutputViper()
	viper.Set("output", "text")
	viper.Set("query", "[0]")

	out := captureStdout(t, func() { TextRenderer{}.Render(sampleData()) })
	want := "id\tsrv_1\nname\talpha\nstatus\ton\n"
	if out != want {
		t.Fatalf("object: got %q, want %q", out, want)
	}
}

func TestWriteTextNestedValuesAreJSONEncoded(t *testing.T) {
	var buf bytes.Buffer
	v := []interface{}{
		map[string]interface{}{
			"name":    "ci",
			"buckets": []interface{}{map[string]interface{}{"bucket_name": "b-1", "permission": "rw"}},
			"size":    float64(1024),
			"missing": nil,
		},
	}
	if err := writeText(&buf, v); err != nil {
		t.Fatal(err)
	}
	// Sorted columns: buckets, missing, name, size.
	want := `[{"bucket_name":"b-1","permission":"rw"}]` + "\t\tci\t1024\n"
	if buf.String() != want {
		t.Fatalf("nested: got %q, want %q", buf.String(), want)
	}
}

func TestWriteTextEmptyAndNil(t *testing.T) {
	var buf bytes.Buffer
	if err := writeText(&buf, []interface{}{}); err != nil {
		t.Fatal(err)
	}
	if err := writeText(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestGetStaticRendererText(t *testing.T) {
	defer resetOutputViper()
	viper.Set("output", "text")
	viper.Set("output_explicit", true)
	if _, ok := GetStaticRenderer().(TextRenderer); !ok {
		t.Fatalf("GetStaticRenderer() = %T, want TextRenderer", GetStaticRenderer())
	}
	if _, ok := GetRenderer().(TextRenderer); !ok {
		t.Fatalf("GetRenderer() = %T, want TextRenderer", GetRenderer())
	}
}

// zzKeyRow stands in for the access-key create result.
type zzKeyRow struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func (r zzKeyRow) TableRow() outputTable.Row { return outputTable.Row{} }

// TestQueryOnSingleResultNeedsIndex documents the shape --query operates on: a
// command's rows are always a list, so a bare identifier yields null and the
// documented pipes must index the element. The `access-keys create` examples in
// the README and the automation help topic depend on this.
func TestQueryOnSingleResultNeedsIndex(t *testing.T) {
	defer viper.Set("query", "")
	row := []ResponseData{zzKeyRow{AccessKeyID: "AK", SecretAccessKey: "s3cr3t"}}
	cases := map[string]string{
		"secret_access_key":     "",         // a JMESPath identifier on a list
		"[0].secret_access_key": "s3cr3t\n", // the form the docs must use
		"[].secret_access_key":  "s3cr3t\n", // projection, also fine
	}
	for expr, want := range cases {
		viper.Set("query", expr)
		generic, err := structuredData(row)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		var buf bytes.Buffer
		if err := writeText(&buf, generic); err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if buf.String() != want {
			t.Errorf("--query %q rendered %q, want %q", expr, buf.String(), want)
		}
	}
}
