package cli

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func TestOperatingSystemToRow(t *testing.T) {
	const fixture = `{
		"id": "os_123",
		"attributes": {
			"name": "Ubuntu 24.04",
			"slug": "ubuntu_24_04_x64_lts",
			"distro": "ubuntu",
			"version": "24.04",
			"user": "ubuntu"
		}
	}`
	os := &components.OperatingSystemData{}
	if err := json.Unmarshal([]byte(fixture), os); err != nil {
		t.Fatal(err)
	}

	row := operatingSystemToRow(os)
	want := operatingSystemRow{
		ID:      "os_123",
		Slug:    "ubuntu_24_04_x64_lts",
		Name:    "Ubuntu 24.04",
		Distro:  "ubuntu",
		Version: "24.04",
		User:    "ubuntu",
	}
	if row != want {
		t.Errorf("operatingSystemToRow = %+v, want %+v", row, want)
	}
}

func TestOperatingSystemToRow_NilSafety(t *testing.T) {
	if row := operatingSystemToRow(nil); row != (operatingSystemRow{}) {
		t.Errorf("operatingSystemToRow(nil) = %+v, want zero row", row)
	}

	id := "os_456"
	row := operatingSystemToRow(&components.OperatingSystemData{ID: &id})
	if row != (operatingSystemRow{ID: "os_456"}) {
		t.Errorf("operatingSystemToRow without attributes = %+v, want only ID set", row)
	}
}
