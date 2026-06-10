package cli

import (
	"context"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
)

func makeOperationGroupIPsCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "ips",
		Short: "List and inspect IP addresses",
	}

	listCmd, err := makeOperationIPsListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	getCmd, err := makeOperationIPsGetCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(getCmd)

	return cmd, nil
}

func makeOperationIPsListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List IP addresses scoped to a project",
		Example: `  lsh ips list --project=my-project
  lsh ips list --all-projects
  lsh ips list --project=my-project --family=IPv4`,
		Args:         cobra.NoArgs,
		RunE:         runIPsList,
		SilenceUsage: true,
	}

	cmd.Flags().String("project", "", "Project ID or slug to scope by")
	// all-projects is consumed by the root resolveProjectFlag hook
	// (project_flag.go): when set, the --project requirement is skipped and
	// FilterProject stays empty, so the API returns IPs from every project.
	cmd.Flags().Bool("all-projects", false, "List IPs across every project you have access to")
	cmd.Flags().String("server", "", "Filter by server ID")
	cmd.Flags().String("family", "", "Filter by family: IPv4 or IPv6")
	cmd.Flags().String("type", "", "Filter by type: public or private")
	cmd.Flags().String("location", "", "Filter by site slug")

	return cmd, nil
}

func makeOperationIPsGetCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "get <ip-id>",
		Short:        "Retrieve a single IP address by ID",
		Example:      `  lsh ips get ip_xxx`,
		Args:         cobra.ExactArgs(1),
		RunE:         runIPGet,
		SilenceUsage: true,
	}
	return cmd, nil
}

type ipRow struct {
	ID       string `json:"id,omitempty"`
	Address  string `json:"address,omitempty"`
	Family   string `json:"family,omitempty"`
	Type     string `json:"type,omitempty"`
	Project  string `json:"project,omitempty"`
	Region   string `json:"region,omitempty"`
	Assigned string `json:"assigned_to,omitempty"`
}

func (i ipRow) TableRow() table.Row {
	return table.Row{
		"id":       {Value: i.ID, Label: "ID"},
		"address":  {Value: i.Address, Label: "Address"},
		"family":   {Value: i.Family, Label: "Family"},
		"type":     {Value: i.Type, Label: "Type"},
		"project":  {Value: i.Project, Label: "Project"},
		"region":   {Value: i.Region, Label: "Region"},
		"assigned": {Value: i.Assigned, Label: "Assigned To"},
	}
}

func runIPsList(cmd *cobra.Command, _ []string) error {
	project, _ := cmd.Flags().GetString("project")
	server, _ := cmd.Flags().GetString("server")
	familyStr, _ := cmd.Flags().GetString("family")
	typeStr, _ := cmd.Flags().GetString("type")
	location, _ := cmd.Flags().GetString("location")

	req := operations.GetIpsRequest{PageSize: &listPageSize}
	if project != "" {
		req.FilterProject = &project
	}
	if server != "" {
		req.FilterServer = &server
	}
	if location != "" {
		req.FilterLocation = &location
	}
	if familyStr != "" {
		fam, err := parseIPFamily(familyStr)
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		req.FilterFamily = &fam
	}
	if typeStr != "" {
		t, err := parseIPType(typeStr)
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		req.FilterType = &t
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	stopSpinner := tui.StartFetchSpinner("Fetching IPs…")
	defer stopSpinner()

	resp, err := client.IPAddresses.List(ctx, req)
	if err != nil {
		stopSpinner()
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.IPAddresses == nil {
		stopSpinner()
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.IPAddresses.Data))
	for resp != nil && resp.IPAddresses != nil {
		for i := range resp.IPAddresses.Data {
			rows = append(rows, ipToRow(&resp.IPAddresses.Data[i]))
		}
		if resp.Next == nil {
			break
		}
		resp, err = resp.Next()
		if err != nil {
			stopSpinner()
			utils.PrintError(err)
			return nil
		}
	}
	stopSpinner()
	renderer.Render(rows)
	return nil
}

func runIPGet(_ *cobra.Command, args []string) error {
	ipID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.IPAddresses.Get(ctx, ipID, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.IPAddress == nil || resp.IPAddress.Data == nil {
		renderer.Render(nil)
		return nil
	}
	renderer.Render([]renderer.ResponseData{ipToRow(resp.IPAddress.Data)})
	return nil
}

func parseIPFamily(s string) (operations.FilterFamily, error) {
	switch s {
	case "IPv4", "ipv4", "v4":
		return operations.FilterFamilyIPv4, nil
	case "IPv6", "ipv6", "v6":
		return operations.FilterFamilyIPv6, nil
	default:
		return "", &invalidEnumError{field: "family", value: s, allowed: "IPv4, IPv6"}
	}
}

func parseIPType(s string) (operations.FilterType, error) {
	switch strings.ToLower(s) {
	case "public":
		return operations.FilterTypePublic, nil
	case "private":
		return operations.FilterTypePrivate, nil
	default:
		return "", &invalidEnumError{field: "type", value: s, allowed: "public, private"}
	}
}

func ipToRow(ip *components.IPAddressData) ipRow {
	row := ipRow{}
	if ip == nil {
		return row
	}
	if ip.ID != nil {
		row.ID = *ip.ID
	}
	if ip.Attributes == nil {
		return row
	}
	if ip.Attributes.Address != nil {
		row.Address = *ip.Attributes.Address
	}
	if ip.Attributes.Family != nil {
		row.Family = string(*ip.Attributes.Family)
	}
	if ip.Attributes.Type != nil {
		row.Type = string(*ip.Attributes.Type)
	}
	if ip.Attributes.Project != nil && ip.Attributes.Project.Name != nil {
		row.Project = *ip.Attributes.Project.Name
	}
	if ip.Attributes.Region != nil && ip.Attributes.Region.Name != nil {
		row.Region = *ip.Attributes.Region.Name
	}
	if ip.Attributes.Assignment != nil && ip.Attributes.Assignment.Hostname != nil {
		row.Assigned = *ip.Attributes.Assignment.Hostname
	}
	return row
}
