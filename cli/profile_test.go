package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/config"
)

func TestPrintProfileList(t *testing.T) {
	f := &config.File{
		DefaultProfile: "labs",
		Profiles: map[string]config.Profile{
			"labs":  {TeamName: "Labs", TeamSlug: "labs", Email: "a@x.com"},
			"teamb": {TeamName: "Team B", TeamSlug: "teamb", Email: "b@x.com"},
		},
	}

	var buf bytes.Buffer
	printProfileList(&buf, f)
	out := buf.String()

	for _, want := range []string{"PROFILE", "TEAM", "EMAIL", "labs", "teamb", "Labs", "Team B", "* = active profile"} {
		if !strings.Contains(out, want) {
			t.Fatalf("profile list missing %q\n%s", want, out)
		}
	}
	if !strings.Contains(out, "* ") {
		t.Fatalf("expected an active-row marker, got\n%s", out)
	}
}
