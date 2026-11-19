package objectstorage

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
)

// Object is a listed or inspected S3 object (also used for versions).
type Object struct {
	Type           string    `json:"type"`
	Key            string    `json:"key"`
	Size           int64     `json:"size"`
	LastModified   time.Time `json:"last_modified"`
	ETag           string    `json:"etag,omitempty"`
	ContentType    string    `json:"content_type,omitempty"`
	StorageClass   string    `json:"storage_class,omitempty"`
	VersionID      string    `json:"version_id,omitempty"`
	IsLatest       *bool     `json:"is_latest,omitempty"`
	IsDeleteMarker bool      `json:"is_delete_marker,omitempty"`
	// Metadata holds x-amz-meta-* values (stat only).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ObjectFromInfo converts a minio listing entry.
func ObjectFromInfo(info minio.ObjectInfo, withVersions bool) Object {
	o := Object{
		Type:           "object",
		Key:            info.Key,
		Size:           info.Size,
		LastModified:   info.LastModified,
		ETag:           strings.Trim(info.ETag, `"`),
		ContentType:    info.ContentType,
		StorageClass:   info.StorageClass,
		IsDeleteMarker: info.IsDeleteMarker,
	}
	// VersionID is copied whenever the backend returns one; IsLatest is only
	// meaningful in a versions listing (HEAD responses never carry it).
	o.VersionID = info.VersionID
	if withVersions {
		latest := info.IsLatest
		o.IsLatest = &latest
	}
	if info.IsDeleteMarker {
		o.Type = "delete_marker"
	}
	return o
}

func (o Object) TableRow() table.Row {
	row := table.Row{
		"type":          {Label: "Type", Value: strings.ToUpper(shortType(o.Type))},
		"key":           {Label: "Key", Value: o.Key, MaxLength: 80},
		"size":          {Label: "Size", Value: HumanSize(o.Size)},
		"last_modified": {Label: "Last Modified", Value: FormatTime(o.LastModified)},
	}
	if o.VersionID != "" {
		row["version_id"] = table.Cell{Label: "Version ID", Value: o.VersionID}
	}
	if o.ContentType != "" {
		row["content_type"] = table.Cell{Label: "Content Type", Value: o.ContentType}
	}
	if o.ETag != "" {
		row["etag"] = table.Cell{Label: "ETag", Value: o.ETag}
	}
	return row
}

func shortType(t string) string {
	switch t {
	case "prefix":
		return "PRE"
	case "delete_marker":
		return "DEL"
	default:
		return "OBJ"
	}
}

// Prefix is a common prefix ("directory") in a non-recursive listing.
type Prefix struct {
	Type   string `json:"type"`
	Prefix string `json:"prefix"`
}

// NewPrefix builds a Prefix entry.
func NewPrefix(p string) Prefix { return Prefix{Type: "prefix", Prefix: p} }

func (p Prefix) TableRow() table.Row {
	return table.Row{
		"type":          {Label: "Type", Value: "PRE"},
		"key":           {Label: "Key", Value: p.Prefix, MaxLength: 80},
		"size":          {Label: "Size", Value: ""},
		"last_modified": {Label: "Last Modified", Value: ""},
	}
}

// TransferResult is one cp/mv operation (or its dry-run plan).
type TransferResult struct {
	Op          string `json:"op"` // upload | download | copy | move
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Size        int64  `json:"size"`
	ETag        string `json:"etag,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
	Error       string `json:"error,omitempty"`
}

func (t TransferResult) TableRow() table.Row {
	status := "ok"
	if t.DryRun {
		status = "dryrun"
	}
	if t.Error != "" {
		status = "error"
	}
	return table.Row{
		"op":          {Label: "Op", Value: t.Op},
		"source":      {Label: "Source", Value: t.Source, MaxLength: 60},
		"destination": {Label: "Destination", Value: t.Destination, MaxLength: 60},
		"size":        {Label: "Size", Value: HumanSize(t.Size)},
		"status":      {Label: "Status", Value: status},
	}
}

// HumanLine renders the aws-style line: `upload: ./a to s3://b/a`.
func (t TransferResult) HumanLine() string {
	prefix := ""
	if t.DryRun {
		prefix = "(dryrun) "
	}
	if t.Error != "" {
		return fmt.Sprintf("%s%s failed: %s to %s: %s", prefix, t.Op, t.Source, t.Destination, t.Error)
	}
	return fmt.Sprintf("%s%s: %s to %s", prefix, t.Op, t.Source, t.Destination)
}

// DeleteResult is one rm/rb deletion (or its dry-run plan).
type DeleteResult struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key,omitempty"`
	VersionID string `json:"version_id,omitempty"`
	Deleted   bool   `json:"deleted"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (d DeleteResult) TableRow() table.Row {
	status := "deleted"
	if d.DryRun {
		status = "dryrun"
	}
	if d.Error != "" {
		status = "error: " + d.Error
	}
	return table.Row{
		"bucket": {Label: "Bucket", Value: d.Bucket},
		"key":    {Label: "Key", Value: d.Key, MaxLength: 80},
		"status": {Label: "Status", Value: status},
	}
}

// HumanLine renders `delete: s3://b/k` (or `remove_bucket: s3://b`).
func (d DeleteResult) HumanLine() string {
	prefix := ""
	if d.DryRun {
		prefix = "(dryrun) "
	}
	target := "s3://" + d.Bucket
	verb := "remove_bucket"
	if d.Key != "" {
		target += "/" + d.Key
		verb = "delete"
	}
	if d.VersionID != "" {
		target += " (version " + d.VersionID + ")"
	}
	if d.Error != "" {
		return fmt.Sprintf("%s%s failed: %s: %s", prefix, verb, target, d.Error)
	}
	return fmt.Sprintf("%s%s: %s", prefix, verb, target)
}

// PresignResult is the output of `presign`.
type PresignResult struct {
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (p PresignResult) TableRow() table.Row {
	return table.Row{
		"url":        {Label: "URL", Value: p.URL},
		"method":     {Label: "Method", Value: p.Method},
		"expires_at": {Label: "Expires At", Value: FormatTime(p.ExpiresAt)},
	}
}

// AsResponseData adapts a slice of any ResponseData implementation.
func AsResponseData[T renderer.ResponseData](items []T) []renderer.ResponseData {
	out := make([]renderer.ResponseData, 0, len(items))
	for _, it := range items {
		out = append(out, it)
	}
	return out
}

// FormatTime renders timestamps the way `aws s3 ls` does, in local time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// HumanSize renders sizes exactly like `aws s3 ls --human-readable`:
// "1 Byte", "12 Bytes", then "1.0 KiB", "3.4 MiB"… promoting the unit when the
// rounded value would reach 1024 (so 1048575 bytes is "1.0 MiB").
func HumanSize(n int64) string {
	if n == 1 {
		return "1 Byte"
	}
	if n < 1024 {
		return fmt.Sprintf("%d Bytes", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	unit := 1024.0
	for i, u := range units {
		unit *= 1024
		if math.Round(float64(n)/unit*1024) < 1024 || i == len(units)-1 {
			return fmt.Sprintf("%.1f %s", 1024*float64(n)/unit, u)
		}
	}
	return fmt.Sprintf("%d Bytes", n)
}

// LsSummary renders the --summarize footer for object listings.
func LsSummary(count int, bytes int64, human bool) string {
	size := fmt.Sprintf("%d", bytes)
	if human {
		size = HumanSize(bytes)
	}
	return fmt.Sprintf("\nTotal Objects: %d\n   Total Size: %s", count, size)
}
