package s3

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// BucketRow wraps the SDK bucket payload so the shared renderer can print it
// (-o table|json|yaml|csv, --query). JSON output is the API document as-is.
type BucketRow struct {
	components.ObjectStorageData
	// Site is the Latitude site slug when known (the SDK model drops it).
	Site string `json:"-"`
	// resolved is set when the row came from a resolved bucket. It carries the
	// fields that exist without an API document (endpoint-override mode).
	resolved *objectstorage.Bucket
}

// MarshalJSON emits the API document plus the site slug (which the SDK model
// drops) so `-o json` carries everything `stat` shows.
func (m *BucketRow) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(m.ObjectStorageData)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(raw, &doc); unmarshalErr != nil {
		doc = nil
	}
	if len(doc) == 0 && m.resolved != nil {
		// No API payload (endpoint-override mode): describe the bucket from
		// what was resolved locally instead of emitting an empty object.
		return json.Marshal(map[string]string{
			"name":           m.resolved.Name,
			"bucket_name":    m.resolved.BucketName,
			"endpoint":       m.resolved.Endpoint,
			"signing_region": m.resolved.SigningRegion,
		})
	}
	if m.Site == "" || doc == nil {
		return raw, nil
	}
	site, _ := json.Marshal(m.Site)
	doc["site"] = site
	return json.Marshal(doc)
}

// NewBucketRow builds a row from a resolved bucket.
func NewBucketRow(b *objectstorage.Bucket) *BucketRow {
	return &BucketRow{ObjectStorageData: b.Data, Site: b.Site, resolved: b}
}

// BucketRows converts SDK payloads into renderable rows.
func BucketRows(data []components.ObjectStorageData, sites map[string]string) []renderer.ResponseData {
	out := make([]renderer.ResponseData, 0, len(data))
	for i := range data {
		row := &BucketRow{ObjectStorageData: data[i]}
		if data[i].ID != nil && sites != nil {
			row.Site = sites[*data[i].ID]
		}
		out = append(out, row)
	}
	return out
}

func (m *BucketRow) TableRow() table.Row {
	b := objectstorage.BucketFromData(m.ObjectStorageData)
	site := m.Site
	if site == "" {
		site = b.Site
	}
	if site == "" {
		site = b.City
	}
	created := ""
	if b.CreatedAt != nil {
		created = objectstorage.FormatTime(*b.CreatedAt)
	}
	return table.Row{
		"id":            {Label: "ID", Value: b.ID},
		"name":          {Label: "Name", Value: b.Name},
		"bucket_name":   {Label: "Bucket Name", Value: b.BucketName},
		"project":       {Label: "Project", Value: b.ProjectRef()},
		"storage_class": {Label: "Class", Value: b.StorageClass},
		"region":        {Label: "Site", Value: site},
		"endpoint":      {Label: "Endpoint", Value: b.Endpoint, MaxLength: 45},
		"versioning":    {Label: "Versioning", Value: yesNo(b.Versioning)},
		"locking":       {Label: "Locking", Value: lockingLabel(b)},
		"created_at":    {Label: "Created At", Value: created},
	}
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func lockingLabel(b *objectstorage.Bucket) string {
	if !b.Locking {
		return "no"
	}
	mode := strings.ToUpper(b.RetentionMode)
	if mode == "" || mode == "NONE" {
		return "yes"
	}
	if b.RetentionDays > 0 {
		return fmt.Sprintf("%s (%dd)", mode, b.RetentionDays)
	}
	return mode
}
