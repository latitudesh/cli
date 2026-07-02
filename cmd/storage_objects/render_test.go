package storage_objects

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const bucketsFixture = `{
  "data": [
    {
      "id": "bucket_abc123",
      "type": "object_storages",
      "attributes": {
        "name": "my-bucket",
        "bucket_name": "my-bucket-xyz",
        "storage_class": "high_performance",
        "endpoint": "https://s3.example.com",
        "created_at": "2026-06-01T12:00:00Z",
        "region": {"id": "SAO2", "city": "Sao Paulo", "country": "Brazil"}
      }
    }
  ]
}`

// TestBucketsPayloadDecoding pins the SDK's typed list envelope to the API
// shape and asserts the rendered row for the first bucket.
func TestBucketsPayloadDecoding(t *testing.T) {
	var payload components.ObjectStorages
	if err := json.Unmarshal([]byte(bucketsFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal buckets fixture: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(payload.Data))
	}

	bucket := Bucket{ObjectStorageData: payload.Data[0]}
	row := bucket.TableRow()

	expectations := map[string]string{
		"id":            "bucket_abc123",
		"name":          "my-bucket",
		"bucket_name":   "my-bucket-xyz",
		"region":        "SAO2",
		"storage_class": "high_performance",
		"endpoint":      "https://s3.example.com",
		"created_at":    "2026-06-01T12:00:00Z",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
	// Live gets return region without an id ({city, country}); the cell must
	// fall back to the city so the column is not blank.
	city := "Miami"
	noID := Bucket{ObjectStorageData: components.ObjectStorageData{
		Attributes: &components.ObjectStorageDataAttributes{
			Region: &components.ObjectStorageDataRegion{City: &city},
		},
	}}
	if got := noID.TableRow()["region"].Value; got != "Miami" {
		t.Errorf("region fallback = %q, want Miami", got)
	}
}

func TestBucketTableRowEmpty(t *testing.T) {
	bucket := Bucket{ObjectStorageData: components.ObjectStorageData{}}
	row := bucket.TableRow()
	if row["name"].Value != "" {
		t.Errorf("expected blank cells for empty bucket, got %+v", row)
	}
}

func TestGetAndDeleteArgs(t *testing.T) {
	if err := NewGetCmd().Args(NewGetCmd(), []string{}); err == nil {
		t.Error("get: expected error with no args")
	}
	if err := NewGetCmd().Args(NewGetCmd(), []string{"id1"}); err != nil {
		t.Errorf("get: unexpected error with one arg: %v", err)
	}
	if err := NewDeleteCmd().Args(NewDeleteCmd(), []string{"a", "b"}); err == nil {
		t.Error("delete: expected error with two args")
	}
}
