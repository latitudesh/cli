package sdkcoverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func miniSurface(t *testing.T) Surface {
	t.Helper()
	s, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func loadYAML(t *testing.T, body string) Manifest {
	t.Helper()
	path := filepath.Join(t.TempDir(), ManifestFile)
	if err := os.WriteFile(path, []byte("version: 1\nsdk_module: "+SDKModulePath+"\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	return m
}

const okManifest = `
groups:
  Servers:
    commands:
      List: servers list
      Get: [servers get, servers create]
      Create: servers create
    excluded:
      GetDeployConfig: {reason: product_decision, notes: dashboard only}
  Projects:
    commands:
      List: projects list
  Projects.SSHKeys:
    commands:
      PostProjectSSHKey: projects ssh-keys create
  Firewalls:
    commands:
      List: firewalls list
  Firewalls.Assignments:
    commands:
      Create: firewalls assignments create
  Billing:
    command_group: servers billing
  Lks:
    excluded_group: {reason: api_constraint, notes: preview}
  Backups:
    excluded_group: {reason: product_decision, notes: fixture}
commands_without_sdk:
  - login
`

var leaves = []string{
	"firewalls assignments create", "firewalls list", "login",
	"projects list", "projects ssh-keys create",
	"servers create", "servers get", "servers list",
}

func TestReconcileCleanManifestHasNoViolations(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, okManifest), leaves)
	if !report.OK() {
		t.Fatalf("unexpected violations: %v", report.Violations)
	}
	c := report.Counts()
	// Lks is excluded as a whole group (2 methods); Billing has no command
	// but its command_group nests under the existing "servers" family, so it
	// is pending without being a new top-level command group.
	if c.Methods != 19 || c.Covered != 7 || c.Excluded != 11 || c.Pending != 1 || c.NewGroups != 3 || c.NewCommandGroups != 0 {
		t.Errorf("counts = %+v", c)
	}
	pending := report.PendingGroups()
	if len(pending) != 1 || pending[0].Name != "Billing" {
		names := []string{}
		for _, g := range pending {
			names = append(names, g.Name)
		}
		t.Fatalf("pending groups = %v, want [Billing]", names)
	}
	if !pending[0].NewGroup() || pending[0].NewCommandGroup() || pending[0].CommandGroup() != "servers billing" {
		t.Errorf("Billing: NewGroup=%v NewCommandGroup=%v CommandGroup=%q", pending[0].NewGroup(), pending[0].NewCommandGroup(), pending[0].CommandGroup())
	}
	// A covered group infers its command group from its mapped commands.
	for _, g := range report.Groups {
		if g.Name == "Servers" && g.CommandGroup() != "servers" {
			t.Errorf("Servers.CommandGroup() = %q, want servers", g.CommandGroup())
		}
	}
}

func TestReconcileReportsEveryContradiction(t *testing.T) {
	manifest := loadYAML(t, `
groups:
  Servers:
    commands:
      List: servers lst
      Lists: servers list
      Get: servers get
    excluded:
      Get: {reason: product_decision}
      Create: {reason: because}
      GetDeployConfig: {}
  Ghost:
    commands:
      List: servers create
  Lks:
    excluded_group: {}
    excluded:
      ListLksClusters: {reason: deprecated}
commands_without_sdk:
  - servers get
  - nope
`)
	report := Reconcile(miniSurface(t), manifest, leaves)

	wants := []string{
		`commands.List names "servers lst"`,              // path not registered
		`commands.Lists: the SDK no longer exposes`,      // method gone
		`Get is both mapped under commands and listed`,   // mapped and excluded
		`excluded.Create has unknown reason "because"`,   // bad reason
		`excluded.GetDeployConfig needs a reason`,        // missing reason
		`Ghost: listed in sdk-coverage.yaml but the SDK`, // group gone
		`commands_without_sdk lists "servers get", but`,  // overlap
		`commands_without_sdk names "nope"`,              // unknown leaf
		`registers "firewalls list" but no SDK method`,   // unclaimed leaf
		`Lks: excluded_group needs a reason`,             // group exclusion without reason
		`Lks: excluded_group already covers`,             // group + per-method exclusion
	}
	joined := ""
	for _, v := range report.Violations {
		joined += v.String() + "\n"
	}
	for _, w := range wants {
		if !strings.Contains(joined, w) {
			t.Errorf("missing violation containing %q\nall violations:\n%s", w, joined)
		}
	}
}

func TestLoadManifestRejectsUnknownKeysAndWrongIdentity(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"typo":    "version: 1\nsdk_module: " + SDKModulePath + "\ngroupz: {}\n",
		"version": "version: 2\nsdk_module: " + SDKModulePath + "\n",
		"module":  "version: 1\nsdk_module: example.com/other\n",
	}
	for name, body := range cases {
		path := filepath.Join(dir, name+".yaml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadManifest(path); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLeafCommandsSkipsHiddenAndHelp(t *testing.T) {
	root := &cobra.Command{Use: "lsh"}
	servers := &cobra.Command{Use: "servers"}
	servers.AddCommand(&cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}})
	servers.AddCommand(&cobra.Command{Use: "secret", Hidden: true, Run: func(*cobra.Command, []string) {}})
	servers.AddCommand(&cobra.Command{Use: "old", Deprecated: "gone", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(servers)
	root.AddCommand(&cobra.Command{Use: "login", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(&cobra.Command{Use: "help", Run: func(*cobra.Command, []string) {}})
	// A runnable parent is a command in its own right as well as a group:
	// `s3 configure` saves an access key and has an `export` child.
	configure := &cobra.Command{Use: "configure", Run: func(*cobra.Command, []string) {}}
	configure.AddCommand(&cobra.Command{Use: "export", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(configure)

	got := LeafCommands(root)
	want := []string{"configure", "configure export", "login", "servers list"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("LeafCommands = %v, want %v", got, want)
	}
}

// Methods that are one operation behind two routes collapse into a single
// suggested command carrying both method names.
func TestPendingSuggestionsMergeNestedRoutes(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, "groups:\n  Backups:\n    command_group: virtual-machines backups\n"), nil)
	var backups GroupReport
	for _, g := range report.Groups {
		if g.Name == "Backups" {
			backups = g
		}
	}
	got := map[string][]string{}
	for _, s := range backups.PendingSuggestions() {
		got[s.Command] = s.Methods
	}
	want := map[string][]string{
		"virtual-machines backups create": {"Create", "CreateForVirtualMachine"},
		"virtual-machines backups get":    {"Get"},
		"virtual-machines backups list":   {"List", "ListForVirtualMachine"},
	}
	if len(got) != len(want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
	for cmd, methods := range want {
		if strings.Join(got[cmd], ",") != strings.Join(methods, ",") {
			t.Errorf("%s = %v, want %v", cmd, got[cmd], methods)
		}
	}
}

// A `suggested` entry wins over the automatic name, and two methods given the
// same suggestion fold into one command.
func TestSuggestedOverridesAutomaticNaming(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Backups:
    command_group: elastic-ips
    suggested:
      Create: elastic-ips attach
      List: elastic-ips get
      Get: elastic-ips get
`), nil)
	var backups GroupReport
	for _, g := range report.Groups {
		if g.Name == "Backups" {
			backups = g
		}
	}
	got := map[string][]string{}
	for _, s := range backups.PendingSuggestions() {
		got[s.Command] = s.Methods
	}
	if strings.Join(got["elastic-ips attach"], ",") != "Create,CreateForVirtualMachine" {
		t.Errorf("attach = %v, want Create plus its nested route", got["elastic-ips attach"])
	}
	if strings.Join(got["elastic-ips get"], ",") != "Get,List,ListForVirtualMachine" {
		t.Errorf("get = %v, want the two suggested methods plus List's nested route", got["elastic-ips get"])
	}
}

func TestSuggestedContradictions(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    commands:
      List: servers list
    suggested:
      List: servers ls
      Nope: servers nope
      Get: "  "
      GetDeployConfig: servers list
`), []string{"servers list"})
	joined := ""
	for _, v := range report.Violations {
		joined += v.String() + "\n"
	}
	for _, w := range []string{
		"suggested.List: the method is already mapped",
		"suggested.Nope: the SDK no longer exposes",
		"suggested.Get is empty",
		`suggested.GetDeployConfig names "servers list", which the CLI already registers`,
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("missing violation containing %q\nall violations:\n%s", w, joined)
		}
	}
}

// An `extends` entry grows a command the CLI already has: the method stays
// pending, every renderer marks the command as one to extend rather than add,
// and two groups growing the same command are not a collision.
func TestExtendsGrowsAnExistingCommand(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    commands:
      List: servers list
    extends:
      Get: servers list
      GetDeployConfig: servers list
  Backups:
    command_group: servers backups
    extends:
      Get: servers list
`), []string{"servers list"})
	if !report.OK() {
		t.Fatalf("unexpected violations: %v", report.Violations)
	}
	if c := report.Counts(); c.Covered != 1 {
		t.Errorf("Covered = %d, want 1: an extension does not cover its methods", c.Covered)
	}

	cmds, _ := report.PendingByRoot()
	got := map[string]PendingCommand{}
	for _, p := range cmds {
		got[p.Group+" | "+p.Command] = p
	}
	for key, methods := range map[string]string{
		"Servers | servers list": "Get,GetDeployConfig",
		"Backups | servers list": "Get",
	} {
		p, ok := got[key]
		if !ok || !p.Extends || strings.Join(p.Methods, ",") != methods {
			t.Errorf("%s = %+v (found %v), want an extension of %s", key, p, ok, methods)
		}
	}
	if p := got["Servers | servers create"]; p.Command == "" || p.Extends {
		t.Errorf("servers create = %+v, want a command to add", p)
	}

	if text := report.Text(""); !strings.Contains(text, "list (extend)") {
		t.Errorf("text report does not mark the extension:\n%s", text)
	}
	if md := report.Markdown(""); !strings.Contains(md, "| `servers list` *(extend)* | Servers.Get, Servers.GetDeployConfig |") {
		t.Errorf("markdown report does not mark the extension:\n%s", md)
	}
	js, err := report.JSON("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"extends": true`) || !strings.Contains(string(js), `"extends": false`) {
		t.Errorf("JSON report must carry extends on every command:\n%s", js)
	}
}

// A name the manifest gives a method (suggested or extends) survives route
// merging even when the SDK lists the other route first: the automatically
// named route joins the named one, not the other way round.
func TestExplicitNamesSurviveRouteMerging(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Backups:
    command_group: virtual-machines backups
    suggested:
      CreateForVirtualMachine: virtual-machines backups snapshot
    extends:
      ListForVirtualMachine: virtual-machines get
`), []string{"virtual-machines get"})
	var backups GroupReport
	for _, g := range report.Groups {
		if g.Name == "Backups" {
			backups = g
		}
	}
	got := map[string]Suggestion{}
	for _, s := range backups.PendingSuggestions() {
		got[s.Command] = s
	}
	if s := got["virtual-machines backups snapshot"]; strings.Join(s.Methods, ",") != "CreateForVirtualMachine,Create" || s.Extends {
		t.Errorf("snapshot = %+v, want the suggested route plus Create's top-level route", s)
	}
	if s := got["virtual-machines get"]; strings.Join(s.Methods, ",") != "ListForVirtualMachine,List" || !s.Extends {
		t.Errorf("virtual-machines get = %+v, want an extension carrying both list routes", s)
	}
}

// Covering a resource's CRUD must not move what is left of it: NodePool is a
// resource because the SDK has CreateLksNodePool and ListLksNodePools, whether
// or not they are mapped yet, so the pending node listing stays under
// `lks node-pools`.
func TestPendingNamesStableAsCoverageGrows(t *testing.T) {
	nodesCommand := func(body string, leaves []string) string {
		report := Reconcile(miniSurface(t), loadYAML(t, body), leaves)
		for _, g := range report.Groups {
			if g.Name != "Lks" {
				continue
			}
			for _, s := range g.PendingSuggestions() {
				for _, m := range s.Methods {
					if m == "ListLksNodes" {
						return s.Command
					}
				}
			}
		}
		return ""
	}
	before := nodesCommand("groups:\n  Lks:\n    command_group: lks\n", nil)
	after := nodesCommand(`
groups:
  Lks:
    command_group: lks
    commands:
      CreateLksNodePool: lks node-pools create
      ListLksNodePools: lks node-pools list
`, []string{"lks node-pools create", "lks node-pools list"})
	if before != "lks node-pools nodes list" || after != before {
		t.Errorf("ListLksNodes = %q before mapping node pools and %q after, want lks node-pools nodes list both times", before, after)
	}
}

// The inferred command group does not depend on SDK method order: the
// shortest parent of the mapped commands wins, ties alphabetically.
func TestCommandGroupPrefersShortestParent(t *testing.T) {
	for _, tc := range []struct{ create, list, want string }{
		// Create comes first in the SDK; its deeper parent must not win.
		{"servers actions create", "servers list", "servers"},
		{"zeta create", "alpha list", "alpha"},
	} {
		report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    commands:
      Create: `+tc.create+`
      List: `+tc.list+`
`), []string{tc.create, tc.list})
		for _, g := range report.Groups {
			if g.Name == "Servers" && g.CommandGroup() != tc.want {
				t.Errorf("mapped %q and %q: CommandGroup() = %q, want %q", tc.create, tc.list, g.CommandGroup(), tc.want)
			}
		}
	}
}

func TestExtendsContradictions(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    commands:
      List: servers list
    excluded:
      GetDeployConfig: {reason: product_decision}
    suggested:
      Create: servers make
    extends:
      List: servers list
      Nope: servers list
      Get: "  "
      Create: servers list
      GetDeployConfig: servers list
  Firewalls:
    extends:
      List: firewalls ls
  Lks:
    excluded_group: {reason: api_constraint}
    extends:
      ListLksSites: servers list
`), []string{"servers list"})
	joined := ""
	for _, v := range report.Violations {
		joined += v.String() + "\n"
	}
	for _, w := range []string{
		"Servers: extends.List: the method is already mapped under commands, so the extension is done",
		"Servers: extends.Nope: the SDK no longer exposes",
		"Servers: extends.Get is empty",
		"Servers: extends.Create: the method also has a suggested command",
		"Servers: extends.GetDeployConfig: the method is excluded",
		`Firewalls: extends.List names "firewalls ls", which the CLI does not register`,
		"Lks: extends.ListLksSites: the method is excluded",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("missing violation containing %q\nall violations:\n%s", w, joined)
		}
	}
}

// Two covered SDK groups filed under one command group must not propose the
// same command (a new group carries its own noun and cannot collide); the
// gate names both so the manifest can separate them. This is the
// Teams.Get / UserProfile.Get case.
func TestReconcileReportsSuggestionCollisionsAcrossGroups(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    command_group: machines
    commands:
      Create: machines create
  Backups:
    command_group: machines
    commands:
      Create: machines create
`), []string{"machines create"})
	joined := ""
	for _, v := range report.Violations {
		joined += v.String() + "\n"
	}
	if !strings.Contains(joined, "machines list would be suggested for both Backups and Servers") {
		t.Errorf("missing collision violation; got:\n%s", joined)
	}
	// Resolving it through `suggested` clears the violation.
	fixed := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    command_group: machines
    commands:
      Create: machines create
  Backups:
    command_group: machines
    commands:
      Create: machines create
    suggested:
      List: machines backups list
      ListForVirtualMachine: machines backups list
      Get: machines backups get
`), []string{"machines create"})
	for _, v := range fixed.Violations {
		if strings.Contains(v.String(), "would be suggested for both") {
			t.Errorf("collision still reported after suggested override: %s", v)
		}
	}
}

// One SDK group can feed several command groups: the report organises
// pending commands by the command group they land under, not by SDK group,
// and a suggestion under a new root counts that root as new.
func TestPendingByRootFollowsSuggestedPrefix(t *testing.T) {
	report := Reconcile(miniSurface(t), loadYAML(t, `
groups:
  Servers:
    commands:
      List: servers list
    suggested:
      GetDeployConfig: lks plans list
  Backups: {excluded_group: {reason: product_decision}}
  Billing: {excluded_group: {reason: product_decision}}
  Firewalls: {excluded_group: {reason: product_decision}}
  Firewalls.Assignments: {excluded_group: {reason: product_decision}}
  Lks: {excluded_group: {reason: product_decision}}
  Projects: {excluded_group: {reason: product_decision}}
  Projects.SSHKeys: {excluded_group: {reason: product_decision}}
`), []string{"servers list"})
	cmds, newRoot := report.PendingByRoot()
	var got []string
	for _, p := range cmds {
		got = append(got, p.Root()+" | "+p.Command+" | "+p.Group)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"lks | lks plans list | Servers", "servers | servers create | Servers", "servers | servers get | Servers"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if !newRoot["lks"] || newRoot["servers"] {
		t.Errorf("newRoot = %v, want lks new and servers existing", newRoot)
	}
	// The suggested command under a new root sorts first.
	if len(cmds) == 0 || cmds[0].Command != "lks plans list" {
		t.Errorf("first pending = %q, want the new-root suggestion", cmds[0].Command)
	}
	if c := report.Counts(); c.NewCommandGroups != 1 {
		t.Errorf("NewCommandGroups = %d, want 1 (lks)", c.NewCommandGroups)
	}
}
