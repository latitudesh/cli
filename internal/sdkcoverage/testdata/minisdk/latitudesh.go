// A frozen miniature of the latitudesh-go-sdk root package, used to pin the
// parser and the naming helper against the SDK's real quirks without depending
// on any particular version being present in the module cache.
//
// This tree lives under testdata/ so the go tool ignores it; it only ever has
// to be parseable, not buildable.
package latitudeshgosdk

type Latitudesh struct {
	SDKVersion string

	Servers   *Servers
	Projects  *Projects
	Firewalls *Firewalls
	Lks       *Lks
	Billing   *Billing
	Backups   *Backups

	sdkConfiguration config.SDKConfiguration
	hooks            *hooks.Hooks
}

type Servers struct {
	rootSDK *Latitudesh
}

func (s *Servers) List()                {}
func (s *Servers) Get()                 {}
func (s *Servers) Create()              {}
func (s *Servers) GetDeployConfig()     {}
func (s *Servers) unexportedIsSkipped() {}

// Projects nests a subgroup whose field name differs from its type name, so
// methods must be resolved by type.
type Projects struct {
	SSHKeys *LatitudeshProjectsSSHKeys

	rootSDK *Latitudesh
}

func (s *Projects) List() {}

type LatitudeshProjectsSSHKeys struct {
	rootSDK *Latitudesh
}

func (s *LatitudeshProjectsSSHKeys) PostProjectSSHKey() {}

type Firewalls struct {
	Assignments *Assignments

	rootSDK *Latitudesh
}

func (s *Firewalls) List() {}

type Assignments struct {
	rootSDK *Latitudesh
}

func (s *Assignments) Create() {}

type Lks struct {
	rootSDK *Latitudesh
}

func (s *Lks) ListLksClusters()   {}
func (s *Lks) CreateLksNodePool() {}

// Nodes are listed per node pool: the route nests them under nodepools, so
// the suggested command is `lks node-pools nodes list`.
func (s *Lks) ListLksNodePools() {
	opURL, _ := utils.GenerateURL(ctx, baseURL, "/lks/clusters/{cluster_id}/nodepools", request, nil)
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}

func (s *Lks) ListLksNodes() {
	opURL, _ := utils.GenerateURL(ctx, baseURL, "/lks/clusters/{cluster_id}/nodepools/{node_pool_id}/nodes", request, nil)
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}

func (s *Lks) ListLksSites() {
	opURL, _ := url.JoinPath(baseURL, "/lks/sites")
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}

type Billing struct {
	rootSDK *Latitudesh
}

func (s *Billing) ListUsage() {}

// Backups exposes the same operations through a top-level and a nested
// route, the shape the real VirtualMachineBackups group has. The bodies carry
// the literal verb and path the parser extracts.
type Backups struct {
	rootSDK *Latitudesh
}

func (s *Backups) List() {
	opURL, _ := url.JoinPath(baseURL, "/virtual_machine_backups")
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}

func (s *Backups) ListForVirtualMachine() {
	opURL, _ := utils.GenerateURL(ctx, baseURL, "/virtual_machines/{virtual_machine_id}/backups", request, nil)
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}

func (s *Backups) Create() {
	opURL, _ := url.JoinPath(baseURL, "/virtual_machine_backups")
	_, _ = http.NewRequestWithContext(ctx, "POST", opURL, nil)
}

func (s *Backups) CreateForVirtualMachine() {
	opURL, _ := utils.GenerateURL(ctx, baseURL, "/virtual_machines/{virtual_machine_id}/backups", request, nil)
	_, _ = http.NewRequestWithContext(ctx, "POST", opURL, nil)
}

func (s *Backups) Get() {
	opURL, _ := utils.GenerateURL(ctx, baseURL, "/virtual_machine_backups/{id}", request, nil)
	_, _ = http.NewRequestWithContext(ctx, "GET", opURL, nil)
}
