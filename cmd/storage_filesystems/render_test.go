package storage_filesystems

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

const filesystemFixture = `{
  "data": {
    "id": "fs_abc123",
    "type": "filesystems",
    "attributes": {
      "name": "data",
      "size_in_gb": 2000,
      "created_at": "2026-06-01T12:00:00Z",
      "project": {"id": "proj_1", "name": "My Project", "slug": "my-project"}
    }
  }
}`

// TestFilesystemPayloadDecoding pins the SDK's typed create/update envelope to
// the shape the live API returns, and asserts the rendered row.
func TestFilesystemPayloadDecoding(t *testing.T) {
	var body operations.PostStorageFilesystemsResponseBody
	if err := json.Unmarshal([]byte(filesystemFixture), &body); err != nil {
		t.Fatalf("could not unmarshal filesystem fixture: %v", err)
	}
	if body.Data == nil {
		t.Fatal("expected data to be populated")
	}

	fs := Filesystem{FilesystemData: *body.Data}
	row := fs.TableRow()

	expectations := map[string]string{
		"id":         "fs_abc123",
		"name":       "data",
		"size_in_gb": "2000",
		"project":    "my-project",
		"created_at": "2026-06-01T12:00:00Z",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestFilesystemTableRowEmpty(t *testing.T) {
	// A bare FilesystemData (nil attributes) must not panic and yields blanks.
	fs := Filesystem{FilesystemData: components.FilesystemData{}}
	row := fs.TableRow()
	if row["name"].Value != "" || row["size_in_gb"].Value != "" {
		t.Errorf("expected blank cells for empty filesystem, got %+v", row)
	}
}
