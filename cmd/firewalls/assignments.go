package firewalls

import (
	"context"
	"fmt"
	"net/http"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

// NewAssignmentsCmd builds the `firewalls assignments` parent command and wires
// its list/create/delete subcommands.
func NewAssignmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assignments",
		Short: "Manage firewall assignments",
		Long: "Manage the association between firewalls and servers.\n\n" +
			"An assignment attaches a firewall to a server so the firewall's rules\n" +
			"apply to that server's traffic.",
		Example: `  lsh firewalls assignments list --firewall fw_xxxxxxxx
  lsh firewalls assignments create --firewall fw_xxxxxxxx --server sv_xxxxxxxx
  lsh firewalls assignments delete fwasg_xxxxxxxx --firewall fw_xxxxxxxx`,
	}

	cmd.AddCommand(NewAssignmentsListCmd())
	cmd.AddCommand(NewAssignmentsCreateCmd())
	cmd.AddCommand(NewAssignmentsDeleteCmd())

	return cmd
}

// ----------------------------------------------------------------------------
// list
// ----------------------------------------------------------------------------

func NewAssignmentsListCmd() *cobra.Command {
	op := ListAssignmentsOperation{}
	cmd := &cobra.Command{
		Long: "List firewall assignments.\n\n" +
			"With --firewall, lists the assignments of a single firewall. Otherwise\n" +
			"lists all firewall assignments in the team, optionally filtered by --server.\n",
		RunE:  op.run,
		Short: "List firewall assignments",
		Example: `  lsh firewalls assignments list --firewall fw_xxxxxxxx
  lsh firewalls assignments list
  lsh firewalls assignments list --server sv_xxxxxxxx`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	cmd.Flags().String("firewall", "", "List assignments of this firewall ID")
	cmd.Flags().String("server", "", "Filter all assignments by server ID (ignored when --firewall is set)")

	return cmd
}

type ListAssignmentsOperation struct{}

func (o *ListAssignmentsOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	firewallID, _ := cmd.Flags().GetString("firewall")

	assignments := FirewallAssignments{}

	if firewallID != "" {
		response, err := client.Firewalls.ListAssignments(ctx, firewallID, nil, nil, operations.WithRetries(lsh.RetryConfig()))
		if err != nil {
			utils.PrintError(err)
			return err
		}
		if response.FirewallAssignments != nil {
			for i := range response.FirewallAssignments.Data {
				assignments.Data = append(assignments.Data, &FirewallAssignment{FirewallAssignmentData: response.FirewallAssignments.Data[i]})
			}
		}
	} else {
		var filterServer *string
		if cmd.Flags().Changed("server") {
			value, _ := cmd.Flags().GetString("server")
			filterServer = &value
		}
		response, err := client.Firewalls.GetAllFirewallAssignments(ctx, filterServer, nil, nil, operations.WithRetries(lsh.RetryConfig()))
		if err != nil {
			utils.PrintError(err)
			return err
		}
		if response.FirewallAssignments != nil {
			for i := range response.FirewallAssignments.Data {
				assignments.Data = append(assignments.Data, &FirewallAssignment{FirewallAssignmentData: response.FirewallAssignments.Data[i]})
			}
		}
	}

	if !lsh.Debug {
		utils.Render(assignments.GetData())
	}

	return nil
}

// ----------------------------------------------------------------------------
// create
// ----------------------------------------------------------------------------

func NewAssignmentsCreateCmd() *cobra.Command {
	op := CreateAssignmentOperation{}
	cmd := &cobra.Command{
		Long:    "Assign a server to a firewall so the firewall's rules apply to it.\n",
		RunE:    op.run,
		Short:   "Assign a server to a firewall",
		Example: `  lsh firewalls assignments create --firewall fw_xxxxxxxx --server sv_xxxxxxxx`,
		Use:     "create",
	}

	cmd.Flags().String("firewall", "", "Firewall ID to assign the server to")
	cmd.Flags().String("server", "", "Server ID to attach to the firewall")

	return cmd
}

type CreateAssignmentOperation struct{}

// buildCreateAssignmentRequest validates the flags and assembles the SDK
// request body. Split out so tests can exercise it without a network call.
func buildCreateAssignmentRequest(cmd *cobra.Command) (firewallID string, body operations.CreateFirewallAssignmentFirewallsAssignmentsRequestBody, err error) {
	firewallID, _ = cmd.Flags().GetString("firewall")
	serverID, _ := cmd.Flags().GetString("server")

	if firewallID == "" {
		return "", body, fmt.Errorf("--firewall is required")
	}
	if serverID == "" {
		return "", body, fmt.Errorf("--server is required")
	}

	body = operations.CreateFirewallAssignmentFirewallsAssignmentsRequestBody{
		Data: operations.CreateFirewallAssignmentFirewallsAssignmentsData{
			Type: operations.CreateFirewallAssignmentFirewallsAssignmentsTypeFirewallAssignments,
			Attributes: &operations.CreateFirewallAssignmentFirewallsAssignmentsAttributes{
				ServerID: serverID,
			},
		},
	}

	return firewallID, body, nil
}

func (o *CreateAssignmentOperation) run(cmd *cobra.Command, args []string) error {
	firewallID, body, err := buildCreateAssignmentRequest(cmd)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.Firewalls.Assignments.Create(ctx, firewallID, body, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		fmt.Println(tui.SuccessStyle.Render("✓ Server assigned to firewall successfully!"))

		var fs components.FirewallServer
		if response.FirewallServer != nil {
			fs = *response.FirewallServer
		}
		// The API returns the assignment in a JSON:API data envelope that the
		// current SDK model does not map (its fields come back empty). Fall back
		// to the request inputs so the confirmation shows the association that
		// was created instead of a blank row.
		if fs.Attributes == nil || getStr(fs.Attributes.FirewallID) == "" {
			serverID, _ := cmd.Flags().GetString("server")
			fs.Attributes = &components.FirewallServerAttributes{
				FirewallID: &firewallID,
				ServerID:   &serverID,
			}
		}
		assignment := FirewallServerAssignment{FirewallServer: fs}
		utils.RenderStatic(assignment.GetData())
	}

	return nil
}

// ----------------------------------------------------------------------------
// delete
// ----------------------------------------------------------------------------

func NewAssignmentsDeleteCmd() *cobra.Command {
	op := DeleteAssignmentOperation{}
	cmd := &cobra.Command{
		Long:    "Remove a firewall assignment, detaching the firewall from the server.\n",
		RunE:    op.run,
		Short:   "Delete a firewall assignment",
		Example: `  lsh firewalls assignments delete fwasg_xxxxxxxx --firewall fw_xxxxxxxx`,
		Use:     "delete <assignment_id>",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
	}

	cmd.Flags().String("firewall", "", "Firewall ID the assignment belongs to")

	return cmd
}

type DeleteAssignmentOperation struct{}

func (o *DeleteAssignmentOperation) run(cmd *cobra.Command, args []string) error {
	firewallID, _ := cmd.Flags().GetString("firewall")
	assignmentID := args[0]

	if firewallID == "" {
		return fmt.Errorf("--firewall is required")
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.Firewalls.DeleteAssignment(ctx, firewallID, assignmentID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		// The API answers deletes with 200 or 204 depending on the path.
		status := 0
		if resp.HTTPMeta.Response != nil {
			status = resp.HTTPMeta.Response.StatusCode
		}
		if status == http.StatusOK || status == http.StatusNoContent {
			fmt.Printf("\nFirewall assignment deleted successfully!\n")
		} else {
			fmt.Printf("Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
