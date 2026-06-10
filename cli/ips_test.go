package cli

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestParseIPFamily(t *testing.T) {
	cases := []struct {
		in      string
		want    operations.FilterFamily
		wantErr bool
	}{
		{in: "IPv4", want: operations.FilterFamilyIPv4},
		{in: "ipv4", want: operations.FilterFamilyIPv4},
		{in: "v4", want: operations.FilterFamilyIPv4},
		{in: "IPv6", want: operations.FilterFamilyIPv6},
		{in: "ipv6", want: operations.FilterFamilyIPv6},
		{in: "v6", want: operations.FilterFamilyIPv6},
		{in: "ipv5", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseIPFamily(tc.in)
		if tc.wantErr != (err != nil) {
			t.Errorf("parseIPFamily(%q): err = %v, wantErr = %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("parseIPFamily(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseIPType(t *testing.T) {
	cases := []struct {
		in      string
		want    operations.FilterType
		wantErr bool
	}{
		{in: "public", want: operations.FilterTypePublic},
		{in: "private", want: operations.FilterTypePrivate},
		{in: "Public", wantErr: true},
		{in: "elastic", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseIPType(tc.in)
		if tc.wantErr != (err != nil) {
			t.Errorf("parseIPType(%q): err = %v, wantErr = %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("parseIPType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIPToRow(t *testing.T) {
	const fixture = `{
		"id": "ip_123",
		"attributes": {
			"address": "192.0.2.10",
			"family": "IPv4",
			"type": "Public",
			"project": {"id": "proj_1", "name": "My Project"},
			"region": {"id": "loc_1", "name": "Sao Paulo"},
			"assignment": {"server_id": "sv_1", "hostname": "web-01"}
		}
	}`
	ip := &components.IPAddress{}
	if err := json.Unmarshal([]byte(fixture), ip); err != nil {
		t.Fatal(err)
	}

	row := ipToRow(ip)
	want := ipRow{
		ID:       "ip_123",
		Address:  "192.0.2.10",
		Family:   "IPv4",
		Type:     "Public",
		Project:  "My Project",
		Region:   "Sao Paulo",
		Assigned: "web-01",
	}
	if row != want {
		t.Errorf("ipToRow = %+v, want %+v", row, want)
	}
}

func TestIPToRow_NilSafety(t *testing.T) {
	if row := ipToRow(nil); row != (ipRow{}) {
		t.Errorf("ipToRow(nil) = %+v, want zero row", row)
	}

	id := "ip_456"
	row := ipToRow(&components.IPAddress{ID: &id})
	if row != (ipRow{ID: "ip_456"}) {
		t.Errorf("ipToRow without attributes = %+v, want only ID set", row)
	}
}
