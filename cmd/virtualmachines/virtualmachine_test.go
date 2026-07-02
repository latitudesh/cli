package virtualmachines

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const vmFixture = `{
  "data": [
    {
      "id": "vm_abc",
      "type": "virtual_machines",
      "attributes": {
        "name": "my-vm",
        "status": "Running",
        "primary_ipv4": "203.0.113.10",
        "site": "SAO2",
        "created_at": "2026-06-01T12:00:00Z",
        "plan": {"id": "plan_x", "name": "vm-small"},
        "operating_system": {"slug": "ubuntu-24-04", "name": "Ubuntu 24.04"}
      }
    }
  ]
}`

// TestVirtualMachinePayloadDecoding pins the SDK's typed VM envelope to the
// shape the live API returns, so a schema regression fails here rather than
// rendering empty tables.
func TestVirtualMachinePayloadDecoding(t *testing.T) {
	var payload components.VirtualMachines
	if err := json.Unmarshal([]byte(vmFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal VM fixture: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(payload.Data))
	}

	vm := VirtualMachine{VirtualMachineAttributes: payload.Data[0]}
	row := vm.TableRow()

	expectations := map[string]string{
		"id":               "vm_abc",
		"name":             "my-vm",
		"status":           "Running",
		"plan":             "vm-small",
		"region":           "SAO2",
		"primary_ipv4":     "203.0.113.10",
		"operating_system": "ubuntu-24-04",
		"created_at":       "2026-06-01T12:00:00Z",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestVirtualMachineJSONRoundTrip ensures the wrapper serializes the embedded
// SDK attributes so -o json output is meaningful.
func TestVirtualMachineJSONRoundTrip(t *testing.T) {
	var payload components.VirtualMachines
	if err := json.Unmarshal([]byte(vmFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal VM fixture: %v", err)
	}
	vm := VirtualMachine{VirtualMachineAttributes: payload.Data[0]}

	b, err := json.Marshal(&vm)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["id"] != "vm_abc" {
		t.Errorf("json id = %v, want vm_abc", out["id"])
	}
}
