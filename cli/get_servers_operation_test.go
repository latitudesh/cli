package cli

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func osServer(slug, name string) *components.ServerData {
	os := &components.OperatingSystem{}
	if slug != "" {
		os.Slug = &slug
	}
	if name != "" {
		os.Name = &name
	}
	return &components.ServerData{
		Attributes: &components.ServerDataAttributes{OperatingSystem: os},
	}
}

func TestServerMatchesOS(t *testing.T) {
	srv := osServer("ubuntu_24_04_x64_lts", "Ubuntu 24.04 LTS")

	cases := []struct {
		name   string
		server *components.ServerData
		query  string
		want   bool
	}{
		{"empty query matches all", srv, "", true},
		{"slug substring", srv, "ubuntu", true},
		{"slug exact", srv, "ubuntu_24_04_x64_lts", true},
		{"case insensitive", srv, "UBUNTU", true},
		{"matches by name", srv, "24.04", true},
		{"no match", srv, "debian", false},
		{"nil server", nil, "ubuntu", false},
		{"nil OS, non-empty query", &components.ServerData{Attributes: &components.ServerDataAttributes{}}, "ubuntu", false},
		{"nil OS, empty query still matches", &components.ServerData{Attributes: &components.ServerDataAttributes{}}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serverMatchesOS(tc.server, tc.query); got != tc.want {
				t.Fatalf("serverMatchesOS(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}
