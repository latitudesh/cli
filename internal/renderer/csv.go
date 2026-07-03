package renderer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	outputtable "github.com/latitudesh/lsh/internal/output/table"
)

type CSVRenderer struct{}

func (cr CSVRenderer) Render(data []ResponseData) {
	if err := renderCSV(data); err != nil {
		reportRenderError(err)
	}
}

func renderCSV(data []ResponseData) error {
	generic, err := structuredData(data)
	if err != nil {
		return err
	}

	// Normalize the (possibly queried) result to a list of records.
	var items []interface{}
	switch v := generic.(type) {
	case []interface{}:
		items = v
	case nil:
		items = nil
	default:
		items = []interface{}{v}
	}

	w := csv.NewWriter(os.Stdout)

	if cols := csvColumns(items); len(cols) > 0 {
		// Records are objects: one column per (ordered) key.
		if err := w.Write(cols); err != nil {
			return err
		}
		for _, it := range items {
			rec := make([]string, len(cols))
			if obj, ok := it.(map[string]interface{}); ok {
				for i, c := range cols {
					rec[i] = scalarToString(obj[c])
				}
			} else {
				// A scalar mixed into a list of objects: place it in column 1.
				rec[0] = scalarToString(it)
			}
			if err := w.Write(rec); err != nil {
				return err
			}
		}
	} else {
		// Records are scalars (e.g. a query like `[].id`): single column.
		if err := w.Write([]string{"value"}); err != nil {
			return err
		}
		for _, it := range items {
			if err := w.Write([]string{scalarToString(it)}); err != nil {
				return err
			}
		}
	}

	w.Flush()
	return w.Error()
}

// csvColumns returns the ordered union of keys across all object records,
// using the same preference ordering as the interactive table so CSV columns
// stay stable and predictable. It returns nil when no record is an object
// (i.e. the data is a flat list of scalars).
func csvColumns(items []interface{}) []string {
	seen := make(map[string]struct{})
	var cols []string
	for _, it := range items {
		obj, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		for k := range obj {
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				cols = append(cols, k)
			}
		}
	}
	outputtable.SortColumnsByPreference(cols)
	return cols
}

// scalarToString renders a JSON scalar for a single CSV cell. Nested objects
// and arrays are JSON-encoded so a cell never spans multiple values.
func scalarToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		// JSON numbers decode to float64; render whole numbers without ".0".
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
