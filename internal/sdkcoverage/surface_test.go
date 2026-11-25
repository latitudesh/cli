package sdkcoverage

import (
	"reflect"
	"testing"
)

const miniSDK = "testdata/minisdk"

func TestParseSurfaceFindsTopLevelAndNestedGroups(t *testing.T) {
	surface, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatalf("ParseSurface: %v", err)
	}
	want := []string{"Backups", "Billing", "Firewalls", "Firewalls.Assignments", "Lks", "Projects", "Projects.SSHKeys", "Servers"}
	if got := surface.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("group paths:\n got %v\nwant %v", got, want)
	}
	if n := surface.MethodCount(); n != 19 {
		t.Errorf("MethodCount = %d, want 19", n)
	}
}

// The field Projects.SSHKeys is typed LatitudeshProjectsSSHKeys, so anything
// that resolves methods by field name instead of by type silently reports an
// empty group.
func TestParseSurfaceResolvesNestedGroupByType(t *testing.T) {
	surface, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatalf("ParseSurface: %v", err)
	}
	group := surface.Groups["Projects.SSHKeys"]
	if group.TypeName != "LatitudeshProjectsSSHKeys" {
		t.Errorf("TypeName = %q, want LatitudeshProjectsSSHKeys", group.TypeName)
	}
	if want := []string{"PostProjectSSHKey"}; !reflect.DeepEqual(group.Methods, want) {
		t.Errorf("methods = %v, want %v", group.Methods, want)
	}
}

func TestParseSurfaceSkipsNonGroupsAndUnexported(t *testing.T) {
	surface, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatalf("ParseSurface: %v", err)
	}
	for _, name := range []string{"SDKVersion", "sdkConfiguration", "hooks", "rootSDK", "Latitudesh"} {
		if _, ok := surface.Groups[name]; ok {
			t.Errorf("%q was collected as a group", name)
		}
	}
	for _, m := range surface.Groups["Servers"].Methods {
		if m == "unexportedIsSkipped" {
			t.Error("unexported method was collected")
		}
	}
}

func TestParseSurfaceErrorsWithoutRootClient(t *testing.T) {
	// No Latitudesh struct means the SDK was restructured; that has to be a
	// loud error, not an empty surface that makes the gate pass vacuously.
	if _, err := ParseSurface("testdata"); err == nil {
		t.Fatal("expected an error when the root client type is absent")
	}
}

func TestVersionFromDir(t *testing.T) {
	cases := map[string]string{
		"/home/u/go/pkg/mod/github.com/latitudesh/latitudesh-go-sdk@v1.21.4": "v1.21.4",
		"sdk@v1.22.0-rc.1": "v1.22.0-rc.1",
		"/tmp/sdk@work":    "sdk@work",
		"/tmp/checkout":    "checkout",
	}
	for dir, want := range cases {
		if got := VersionFromDir(dir); got != want {
			t.Errorf("VersionFromDir(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestSingular(t *testing.T) {
	cases := map[string]string{
		"Databases": "Database", "Database": "Database", "Sandboxes": "Sandbox", "Sandbox": "Sandbox",
		"Policies": "Policy", "Metrics": "Metric", "Status": "Status", "Address": "Address", "Lks": "Lk",
	}
	for in, want := range cases {
		if got := singular(in); got != want {
			t.Errorf("singular(%q) = %q, want %q", in, got, want)
		}
	}
}

// A covered group keeps the noun its commands already settled on: BlockStorage
// lives under `volume`, so a pending method is `volume map`, never
// `volume map-storage`.
func TestSuggestSubcommandKeepsSettledNoun(t *testing.T) {
	cases := []struct{ group, groupCommand, method, want string }{
		{"BlockStorage", "volume", "MapVolume", "volume map"},
		{"UserProfile", "teams", "Get", "teams get"},
		{"Servers", "servers", "GetDeployConfig", "servers get-deploy-config"},
	}
	for _, tc := range cases {
		if got := SuggestSubcommand(Group{Name: tc.group}, tc.groupCommand, tc.method); got != tc.want {
			t.Errorf("SuggestSubcommand(%s, %q, %s) = %q, want %q", tc.group, tc.groupCommand, tc.method, got, tc.want)
		}
	}
}

// A noun the SDK flattened into several CRUD method names is a resource and
// becomes a subgroup with plain verbs, the way `kubernetes clusters create`
// and `firewalls assignments create` already read.
func TestSuggestCommandsPromotesResourcesToSubgroups(t *testing.T) {
	lks := Group{Name: "Lks"}
	methods := []string{
		"CreateLksCluster", "GetLksCluster", "ListLksClusters", "UpdateLksCluster", "DeleteLksCluster", "GetLksClusterKubeconfig",
		"CreateLksNodePool", "GetLksNodePool", "ListLksNodePools",
		"ListLksSites", "ListLksNodes", "ListLksAvailableVersions", "ListLksAllocatableServers",
	}
	got := SuggestCommands(lks, "lks", methods, true)
	want := map[string]string{
		"CreateLksCluster":        "lks clusters create",
		"ListLksClusters":         "lks clusters list",
		"GetLksClusterKubeconfig": "lks clusters get-kubeconfig",
		"CreateLksNodePool":       "lks node-pools create",
		"ListLksNodePools":        "lks node-pools list",
		// list alone marks a reference collection: a subgroup, like
		// `kubernetes versions list`; adjectives that qualify the listing
		// drop out of the subgroup's name.
		"ListLksSites":              "lks sites list",
		"ListLksNodes":              "lks nodes list",
		"ListLksAvailableVersions":  "lks versions list",
		"ListLksAllocatableServers": "lks servers list",
	}
	for m, w := range want {
		if got[m] != w {
			t.Errorf("%s = %q, want %q", m, got[m], w)
		}
	}

	// A mass noun the SDK only ever lists in the singular (ListUsage,
	// ListStorage) keeps that spelling: `billing usage list`, not "usages".
	billing := SuggestCommands(Group{Name: "Billing"}, "billing", []string{"ListUsage"}, true)
	if billing["ListUsage"] != "billing usage list" {
		t.Errorf("ListUsage = %q, want billing usage list", billing["ListUsage"])
	}
	plans := SuggestCommands(Group{Name: "Plans"}, "plans", []string{"ListStorage", "GetBandwidth"}, false)
	if plans["ListStorage"] != "plans storage list" {
		t.Errorf("ListStorage = %q, want plans storage list", plans["ListStorage"])
	}

	// A group whose methods are bare (Create, List, Get) takes its noun from
	// the group name, and the group name is where the plural is spelled:
	// VirtualMachineRestores under `virtual-machines backups` yields
	// `backups restores list`, never a singular "restore" subgroup.
	restores := SuggestCommands(Group{Name: "VirtualMachineRestores"}, "virtual-machines backups", []string{"Create", "Get", "List"}, true)
	if restores["List"] != "virtual-machines backups restores list" || restores["Create"] != "virtual-machines backups restores create" {
		t.Errorf("restores = %v", restores)
	}

	// A noun that is only got and updated is a singleton setting of its
	// parent, not a collection: no subgroup.
	servers := SuggestCommands(Group{Name: "Servers"}, "servers", []string{"GetDeployConfig", "UpdateDeployConfig", "List"}, false)
	if servers["GetDeployConfig"] != "servers get-deploy-config" || servers["UpdateDeployConfig"] != "servers update-deploy-config" {
		t.Errorf("deploy config = %v, want hyphenated leaves", servers)
	}

	// The plural get beside the singular get is the list.
	s3 := SuggestCommands(Group{Name: "ObjectStorage"}, "s3", []string{
		"GetStorageBucketLifecycleRule", "GetStorageBucketLifecycleRules", "PostStorageBucketLifecycleRules", "DeleteStorageBucketLifecycleRule",
	}, true)
	if s3["GetStorageBucketLifecycleRules"] != "s3 bucket-lifecycle-rules list" || s3["GetStorageBucketLifecycleRule"] != "s3 bucket-lifecycle-rules get" {
		t.Errorf("lifecycle rules = %v", s3)
	}
}

// A collection whose route nests it under another resource of the group is
// a sub-resource: nodes belong to one node pool.
func TestSuggestCommandsNestsByRoute(t *testing.T) {
	surface, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatal(err)
	}
	lks := surface.Groups["Lks"]
	got := SuggestCommands(lks, "lks", lks.Methods, true)
	want := map[string]string{
		"ListLksNodePools": "lks node-pools list",
		"ListLksNodes":     "lks node-pools nodes list",
		"ListLksSites":     "lks sites list",
		"ListLksClusters":  "lks clusters list",
	}
	for m, w := range want {
		if got[m] != w {
			t.Errorf("%s = %q, want %q", m, got[m], w)
		}
	}
}

func TestSuggestCommand(t *testing.T) {
	lks := Group{Name: "Lks"}
	servers := Group{Name: "Servers"}
	eips := Group{Name: "ElasticIps"}
	baselines := Group{Name: "BaselinesPreview"}
	backups := Group{Name: "VirtualMachineBackups"}
	cases := []struct {
		group        Group
		groupCommand string
		method, want string
	}{
		{lks, "", "ListLksClusters", "lks list-clusters"},
		{lks, "", "CreateLksNodePool", "lks create-node-pool"},
		{servers, "servers", "GetDeployConfig", "servers get-deploy-config"},
		{servers, "servers", "List", "servers list"},
		{eips, "elastic-ips", "ListElasticIPBgpSessions", "elastic-ips list-bgp-sessions"},
		{baselines, "", "GetBaselines", "baselines-preview list"},
		{baselines, "", "GetBaseline", "baselines-preview get"},
		{backups, "virtual-machines backups", "CreateForVirtualMachine", "virtual-machines backups create-for"},
		{backups, "virtual-machines backups", "ListForVirtualMachine", "virtual-machines backups list-for"},
		{Group{Name: "ManagedDatabases"}, "databases", "CreateManagedDatabase", "databases create"},
		{Group{Name: "ManagedDatabases"}, "databases", "ListManagedDatabaseBackups", "databases list-backups"},
		{Group{Name: "SandboxesPreview"}, "sandboxes", "ExecSandbox", "sandboxes exec"},
		{Group{Name: "SandboxesPreview"}, "sandboxes", "GetSandboxes", "sandboxes list"},
		{Group{Name: "Plans.VM"}, "plans", "List", "plans list"},
		{Group{Name: "VirtualMachineRestores"}, "virtual-machines backups", "Create", "virtual-machines backups create-restore"},
		{Group{Name: "VirtualMachineRestores"}, "virtual-machines backups", "ListForBackup", "virtual-machines backups list-restore-for"},
		{Group{Name: "Roles"}, "teams roles", "List", "teams roles list"},
		{Group{Name: "Oddities"}, "", "Frobnicate", "oddities frobnicate"},
	}
	for _, tc := range cases {
		if got := SuggestCommand(tc.group, tc.groupCommand, tc.method); got != tc.want {
			t.Errorf("SuggestCommand(%s, %q, %s) = %q, want %q", tc.group.Name, tc.groupCommand, tc.method, got, tc.want)
		}
	}
}

func TestParseSurfaceExtractsRoutes(t *testing.T) {
	surface, err := ParseSurface(miniSDK)
	if err != nil {
		t.Fatalf("ParseSurface: %v", err)
	}
	backups := surface.Groups["Backups"]
	want := map[string]Route{
		"List":                  {Verb: "GET", Path: "/virtual_machine_backups"},
		"ListForVirtualMachine": {Verb: "GET", Path: "/virtual_machines/{virtual_machine_id}/backups"},
		"Create":                {Verb: "POST", Path: "/virtual_machine_backups"},
		"Get":                   {Verb: "GET", Path: "/virtual_machine_backups/{id}"},
	}
	for m, r := range want {
		if got := backups.Routes[m]; got != r {
			t.Errorf("Routes[%s] = %+v, want %+v", m, got, r)
		}
	}
	// A body without the generated shape records no route.
	if _, ok := surface.Groups["Servers"].Routes["List"]; ok {
		t.Error("Servers.List has no request body and must not record a route")
	}
	if got := (Route{Path: "/virtual_machines/{virtual_machine_id}/backups/{backup_id}/restores"}).Normalized(); got != "/virtual_machines/{}/backups/{}/restores" {
		t.Errorf("Normalized = %q", got)
	}
}

func TestSameOperation(t *testing.T) {
	cases := []struct {
		a, b Route
		want bool
	}{
		{Route{"POST", "/virtual_machine_backups"}, Route{"POST", "/virtual_machines/{virtual_machine_id}/backups"}, true},
		{Route{"GET", "/virtual_machine_restores"}, Route{"GET", "/virtual_machines/{vm}/backups/{b}/restores"}, true},
		// Two nested spellings of the same name do not merge with each other
		// directly; both merge with the top-level "/virtual_machine_restores",
		// which is what folds the three into one suggestion.
		{Route{"GET", "/virtual_machine_backups/{backup_id}/restores"}, Route{"GET", "/virtual_machines/{vm}/backups/{b}/restores"}, false},
		{Route{"GET", "/virtual_machine_backups"}, Route{"POST", "/virtual_machine_backups"}, false}, // list vs create
		{Route{"GET", "/servers"}, Route{"GET", "/servers/{server_id}"}, false},                      // list vs get
		// The same final name under a different parent, with no parent prefix
		// in the top-level spelling, is a different listing.
		{Route{"GET", "/storage/access_keys"}, Route{"GET", "/storage/buckets/{id}/access_keys"}, false},
		{Route{"GET", "/user_data"}, Route{"GET", "/projects/{project_id}/user_data"}, false},
		{Route{"", "/x"}, Route{"GET", "/x"}, false}, // unknown route never merges
	}
	for _, tc := range cases {
		if got := sameOperation(tc.a, tc.b); got != tc.want {
			t.Errorf("sameOperation(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
