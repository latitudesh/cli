package renderer

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// TextRenderer implements the "text" output format, modelled on
// `aws --output text`: values are printed raw, without quotes or structure,
// so a `--query secret_access_key` result can be piped straight into another
// tool (e.g. `gh secret set`).
//
// Shapes, after --query has been applied:
//   - scalar            -> the value on one line, without quotes
//   - list of scalars   -> one value per line
//   - list of objects   -> one tab-separated row per object, keys sorted
//   - object            -> one "key<TAB>value" line per key, keys sorted
//
// Nested values (objects or arrays inside a cell) are JSON-encoded so a row
// never spans multiple lines.
type TextRenderer struct{}

func (tr TextRenderer) Render(data []ResponseData) {
	if err := renderText(data); err != nil {
		reportRenderError(err)
	}
}

func renderText(data []ResponseData) error {
	generic, err := structuredData(data)
	if err != nil {
		return err
	}
	return writeText(os.Stdout, generic)
}

// writeText renders the generic (JSON-shaped) value as plain text.
func writeText(w io.Writer, v interface{}) error {
	switch t := v.(type) {
	case nil:
		return nil
	case []interface{}:
		if len(t) == 0 {
			return nil
		}
		if textHasObjects(t) {
			cols := textColumns(t)
			for _, it := range t {
				if _, err := fmt.Fprintln(w, textRow(it, cols)); err != nil {
					return err
				}
			}
			return nil
		}
		for _, it := range t {
			if _, err := fmt.Fprintln(w, scalarToString(it)); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		keys := sortedKeys(t)
		for _, k := range keys {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", k, scalarToString(t[k])); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := fmt.Fprintln(w, scalarToString(v))
		return err
	}
}

// textHasObjects reports whether any item in the list is an object; a list
// of objects is rendered as rows while a list of scalars is one per line.
func textHasObjects(items []interface{}) bool {
	for _, it := range items {
		if _, ok := it.(map[string]interface{}); ok {
			return true
		}
	}
	return false
}

// textColumns returns the sorted union of keys across every object in the
// list. Sorted (rather than preference-ordered) keys keep the layout
// predictable for callers that cut columns with awk/cut, matching aws.
func textColumns(items []interface{}) []string {
	seen := make(map[string]struct{})
	for _, it := range items {
		obj, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		for k := range obj {
			seen[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(seen))
	for k := range seen {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

// textRow renders one list item as a tab-separated row. A scalar mixed into
// a list of objects is printed as-is.
func textRow(it interface{}, cols []string) string {
	obj, ok := it.(map[string]interface{})
	if !ok {
		return scalarToString(it)
	}
	cells := make([]string, len(cols))
	for i, c := range cols {
		cells[i] = scalarToString(obj[c])
	}
	return strings.Join(cells, "\t")
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
