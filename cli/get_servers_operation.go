package cli

// MANUAL — this command was migrated off the legacy go-swagger client to the
// latitudesh-go-sdk so that `servers list` participates in the same output
// formatting (-o table|json|yaml|csv, --query) and global pagination
// (--page-size/--max-items/--no-paginate) as every other list command.

import (
	"context"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

// makeOperationServersGetServersCmd returns a cmd to handle operation getServers
func makeOperationServersGetServersCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List servers",
		Long:         `Returns a list of all servers belonging to the team.`,
		Args:         cobra.NoArgs,
		RunE:         runOperationServersGetServers,
		SilenceUsage: true,
	}

	registerOperationServersGetServersParamFlags(cmd)

	// MANUAL — keep when regenerating. Lets the user skip the interactive
	// project prompt and list servers from every project. Consumed by the
	// root resolveProjectFlag hook (project_flag.go).
	cmd.Flags().Bool("all-projects", false, "list servers across all projects in the active team")

	return cmd, nil
}

// registerOperationServersGetServersParamFlags registers the user-facing
// filter flags. Names match the previous (generated) command so existing
// scripts keep working.
func registerOperationServersGetServersParamFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	f.String("project", "", "The project ID or Slug to filter by")
	f.String("region", "", "The region Slug to filter by")
	f.String("hostname", "", "The hostname of server to filter by")
	f.String("label", "", "The label of server to filter by")
	f.String("status", "", "The status of server to filter by")
	f.String("plan", "", "The platform/plan name of the server to filter by")
	f.String("tags", "", "The Tags to filter by")
	f.String("created_at_gte", "", "The created at greater than equal date to filter by")
	f.String("created_at_lte", "", "The created at less than equal date to filter by")
	f.Bool("gpu", false, "Filter by the existence of an associated GPU")
	f.Int64("ram_eql", 0, "Filter servers with RAM size (in GB) equal to the provided value")
	f.Int64("ram_gte", 0, "Filter servers with RAM size (in GB) greater than or equal to the provided value")
	f.Int64("ram_lte", 0, "Filter servers with RAM size (in GB) less than or equal to the provided value")
	f.Int64("disk_eql", 0, "Filter servers with disk size in GB equal to the provided value")
	f.Int64("disk_gte", 0, "Filter servers with disk size in GB greater than or equal to the provided value")
	f.Int64("disk_lte", 0, "Filter servers with disk size in GB less than or equal to the provided value")
	f.String("extra_fields[servers]", "", "Extra fields to request (e.g. `credentials`)")

	// operating_system is filtered client-side: the /servers endpoint rejects
	// filter[operating_system] with HTTP 422, so we match against the OS slug
	// or name in the response. TODO(PD-6072): move server-side once the API
	// supports filter[operating_system].
	f.String("operating_system", "", "Filter by operating system slug or name (matched client-side)")
}

// serverRow is the renderer-friendly projection of a server. JSON tags drive
// json/yaml/csv output; TableRow drives the table / interactive view. The
// "hostname" and "ipmi_status" keys are what the interactive renderer uses to
// detect server data and enable the details pane, so they must stay present.
type serverRow struct {
	ID              string `json:"id,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
	PrimaryIPv4     string `json:"primary_ipv4,omitempty"`
	PrimaryIPv6     string `json:"primary_ipv6,omitempty"`
	Location        string `json:"location,omitempty"`
	Status          string `json:"status,omitempty"`
	IpmiStatus      string `json:"ipmi_status,omitempty"`
	Project         string `json:"project,omitempty"`
	Plan            string `json:"plan,omitempty"`
	OperatingSystem string `json:"operating_system,omitempty"`
}

func (s serverRow) TableRow() table.Row {
	return table.Row{
		"id":               {Value: s.ID, Label: "ID"},
		"hostname":         {Value: s.Hostname, Label: "Hostname", MaxLength: 15},
		"primary_ipv4":     {Value: s.PrimaryIPv4, Label: "Primary IPV4"},
		"primary_ipv6":     {Value: s.PrimaryIPv6, Label: "Primary IPV6"},
		"location":         {Value: s.Location, Label: "Location"},
		"status":           {Value: s.Status, Label: "Status"},
		"ipmi_status":      {Value: s.IpmiStatus, Label: "IPMI Status"},
		"project":          {Value: s.Project, Label: "Project"},
		"plan":             {Value: s.Plan, Label: "Plan"},
		"operating_system": {Value: s.OperatingSystem, Label: "OS", MaxLength: 10},
	}
}

// runOperationServersGetServers builds the request from flags, lists servers
// (paginating per the global controls) and renders them.
func runOperationServersGetServers(cmd *cobra.Command, _ []string) error {
	page := pagination.Resolve()
	pageSize := page.PageSize
	pageNumber := int64(1)
	req := operations.GetServersRequest{PageSize: &pageSize, PageNumber: &pageNumber}
	applyServerFilters(cmd, &req)

	// operating_system has no server-side filter (the API 422s), so it is
	// applied client-side against each page. --max-items therefore caps the
	// post-filter count: Walk keeps fetching pages until it has enough matches.
	osFilter, _ := cmd.Flags().GetString("operating_system")

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	stopSpinner := tui.StartFetchSpinner("Fetching servers…")
	defer stopSpinner()

	resp, err := client.Servers.List(ctx, req)
	if err != nil {
		stopSpinner()
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.Servers == nil {
		stopSpinner()
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.Servers.Data))
	result, err := pagination.Walk(resp, page,
		func(r *operations.GetServersResponse) func() (*operations.GetServersResponse, error) { return r.Next },
		func(r *operations.GetServersResponse, limit int) int {
			if r.Servers == nil {
				return 0
			}
			added := 0
			for i := range r.Servers.Data {
				if limit >= 0 && added >= limit {
					break
				}
				sd := &r.Servers.Data[i]
				if !serverMatchesOS(sd, osFilter) {
					continue
				}
				rows = append(rows, serverDataToRow(sd))
				added++
			}
			return added
		},
	)
	if err != nil {
		stopSpinner()
		utils.PrintError(err)
		return nil
	}
	stopSpinner()
	renderer.Render(rows)
	if page.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}
	return nil
}

// applyServerFilters maps the changed filter flags onto the SDK request.
func applyServerFilters(cmd *cobra.Command, req *operations.GetServersRequest) {
	f := cmd.Flags()

	if f.Changed("project") {
		v, _ := f.GetString("project")
		req.FilterProject = &v
	}
	if f.Changed("region") {
		v, _ := f.GetString("region")
		req.FilterRegion = &v
	}
	if f.Changed("hostname") {
		v, _ := f.GetString("hostname")
		req.FilterHostname = &v
	}
	if f.Changed("label") {
		v, _ := f.GetString("label")
		req.FilterLabel = &v
	}
	if f.Changed("status") {
		v, _ := f.GetString("status")
		req.FilterStatus = &v
	}
	if f.Changed("plan") {
		v, _ := f.GetString("plan")
		req.FilterPlan = &v
	}
	if f.Changed("tags") {
		v, _ := f.GetString("tags")
		req.FilterTags = &v
	}
	if f.Changed("created_at_gte") {
		v, _ := f.GetString("created_at_gte")
		req.FilterCreatedAtGte = &v
	}
	if f.Changed("created_at_lte") {
		v, _ := f.GetString("created_at_lte")
		req.FilterCreatedAtLte = &v
	}
	if f.Changed("gpu") {
		v, _ := f.GetBool("gpu")
		req.FilterGpu = &v
	}
	if f.Changed("ram_eql") {
		v, _ := f.GetInt64("ram_eql")
		req.FilterRAMEql = &v
	}
	if f.Changed("ram_gte") {
		v, _ := f.GetInt64("ram_gte")
		req.FilterRAMGte = &v
	}
	if f.Changed("ram_lte") {
		v, _ := f.GetInt64("ram_lte")
		req.FilterRAMLte = &v
	}
	if f.Changed("disk_eql") {
		v, _ := f.GetInt64("disk_eql")
		req.FilterDiskEql = &v
	}
	if f.Changed("disk_gte") {
		v, _ := f.GetInt64("disk_gte")
		req.FilterDiskGte = &v
	}
	if f.Changed("disk_lte") {
		v, _ := f.GetInt64("disk_lte")
		req.FilterDiskLte = &v
	}
	if f.Changed("extra_fields[servers]") {
		v, _ := f.GetString("extra_fields[servers]")
		req.ExtraFieldsServers = &v
	}
}

// serverMatchesOS reports whether a server's operating system matches the
// client-side --operating_system filter. An empty filter matches everything.
// Matching is a case-insensitive substring test against the OS slug and name,
// so "ubuntu" matches "ubuntu_24_04_x64_lts".
func serverMatchesOS(s *components.ServerData, query string) bool {
	if query == "" {
		return true
	}
	if s == nil || s.Attributes == nil || s.Attributes.OperatingSystem == nil {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	os := s.Attributes.OperatingSystem
	if os.Slug != nil && strings.Contains(strings.ToLower(*os.Slug), q) {
		return true
	}
	if os.Name != nil && strings.Contains(strings.ToLower(*os.Name), q) {
		return true
	}
	return false
}

func serverDataToRow(s *components.ServerData) serverRow {
	row := serverRow{}
	if s == nil {
		return row
	}
	if s.ID != nil {
		row.ID = *s.ID
	}
	attr := s.Attributes
	if attr == nil {
		return row
	}
	if attr.Hostname != nil {
		row.Hostname = *attr.Hostname
	}
	if attr.PrimaryIpv4 != nil {
		row.PrimaryIPv4 = *attr.PrimaryIpv4
	}
	if attr.PrimaryIpv6 != nil {
		row.PrimaryIPv6 = *attr.PrimaryIpv6
	}
	if attr.Status != nil {
		row.Status = string(*attr.Status)
	}
	if attr.IpmiStatus != nil {
		row.IpmiStatus = string(*attr.IpmiStatus)
	}
	if attr.Region != nil && attr.Region.Site != nil && attr.Region.Site.Slug != nil {
		row.Location = *attr.Region.Site.Slug
	}
	if attr.Project != nil && attr.Project.Slug != nil {
		row.Project = *attr.Project.Slug
	}
	if attr.Plan != nil && attr.Plan.Name != nil {
		row.Plan = *attr.Plan.Name
	}
	if attr.OperatingSystem != nil && attr.OperatingSystem.Slug != nil {
		row.OperatingSystem = *attr.OperatingSystem.Slug
	}
	return row
}
