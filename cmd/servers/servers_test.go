package servers

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func strp(s string) *string { return &s }

// TestActionsFlagParsing pins the action → SDK enum mapping and rejects
// invalid actions.
func TestActionsFlagParsing(t *testing.T) {
	cases := map[string]struct {
		want operations.CreateServerActionAction
		ok   bool
	}{
		"power_on":  {operations.CreateServerActionActionPowerOn, true},
		"power_off": {operations.CreateServerActionActionPowerOff, true},
		"reboot":    {operations.CreateServerActionActionReboot, true},
		"bogus":     {"", false},
	}
	for action, want := range cases {
		got, ok := validActions[action]
		if ok != want.ok {
			t.Errorf("validActions[%q] ok = %v, want %v", action, ok, want.ok)
		}
		if ok && got != want.want {
			t.Errorf("validActions[%q] = %v, want %v", action, got, want.want)
		}
	}
}

func TestActionsCmdRequiresArg(t *testing.T) {
	cmd := NewRebootCmd()
	if cmd.Args == nil {
		t.Fatal("expected ExactArgs validator on power action command")
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with no server ID, got nil")
	}
	if err := cmd.Args(cmd, []string{"sv_x"}); err != nil {
		t.Errorf("expected no error with one server ID, got %v", err)
	}
}

// TestPowerActionTargets verifies each action waits for the correct terminal
// power state.
func TestPowerActionTargets(t *testing.T) {
	cases := map[string]components.ServerDataStatus{
		"power_on":  components.ServerDataStatusOn,
		"reboot":    components.ServerDataStatusOn,
		"power_off": components.ServerDataStatusOff,
	}
	for action, wantState := range cases {
		want, fail := powerActionTargets(action)
		if len(want) != 1 || want[0] != wantState {
			t.Errorf("powerActionTargets(%q) want = %v, expected [%v]", action, want, wantState)
		}
		if len(fail) != 1 || fail[0] != components.ServerDataStatusFailedDeployment {
			t.Errorf("powerActionTargets(%q) fail = %v, expected [failed_deployment]", action, fail)
		}
	}
}

// TestServerActionModelTableRow pins the rendered columns of a power action.
func TestServerActionModelTableRow(t *testing.T) {
	model := &ServerActionModel{
		ServerID: "sv_x",
		Action:   "reboot",
		Data: &components.ServerActionData{
			ID:         strp("act_1"),
			Attributes: &components.ServerActionAttributes{Status: strp("queued")},
		},
	}
	row := model.TableRow()
	expect := map[string]string{
		"server_id": "sv_x",
		"action":    "reboot",
		"id":        "act_1",
		"status":    "queued",
	}
	for k, want := range expect {
		if got := row[k].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", k, got, want)
		}
	}
}

// TestWaitFlagsRegistered ensures --wait/--timeout are present on the commands
// that should support waiting.
func TestWaitFlagsRegistered(t *testing.T) {
	// Commands that must support waiting.
	if NewRebootCmd().Flags().Lookup("wait") == nil {
		t.Error("reboot: missing --wait flag")
	}
	if NewRescueModeCmd().Flags().Lookup("wait") == nil {
		t.Error("rescue-mode: missing --wait flag")
	}
	if NewExitRescueModeCmd().Flags().Lookup("wait") == nil {
		t.Error("exit-rescue-mode: missing --wait flag")
	}
	// Commands that must NOT have --wait.
	if NewLockCmd().Flags().Lookup("wait") != nil {
		t.Error("lock: unexpected --wait flag")
	}
}
