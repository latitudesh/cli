package firewalls

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	op := UpdateFirewallOperation{}
	cmd := &cobra.Command{
		Long: "Update a firewall's name and/or rules.\n\n" +
			"Providing --rules replaces the firewall's entire rule set. Rules can be\n" +
			"supplied inline as JSON or from a file with @path/to/rules.json.\n",
		RunE:  op.run,
		Short: "Update a firewall",
		Example: `  lsh firewalls update fw_xxxxxxxx --name new-name
  lsh firewalls update fw_xxxxxxxx --rules @rules.json`,
		Use:  "update <id>",
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().String("name", "", "New name for the firewall")
	cmd.Flags().String("rules", "", "Replacement rules as a JSON array, or @path to a JSON file")

	return cmd
}

type UpdateFirewallOperation struct{}

// parseUpdateRulesFlag resolves the --rules flag into SDK update-rule structs.
// A value prefixed with '@' is read from a file; otherwise it is inline JSON.
func parseUpdateRulesFlag(value string) ([]operations.UpdateFirewallFirewallsRules, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	var raw []byte
	if strings.HasPrefix(value, "@") {
		path := strings.TrimPrefix(value, "@")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("could not read rules file %q: %w", path, err)
		}
		raw = content
	} else {
		raw = []byte(value)
	}

	var rules []operations.UpdateFirewallFirewallsRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("could not parse firewall rules JSON: %w", err)
	}

	return rules, nil
}

// buildUpdateFirewallRequest assembles the update request from flags. Only the
// fields the user actually provided are set so an unspecified field is left
// untouched by the API.
func buildUpdateFirewallRequest(cmd *cobra.Command) (*operations.UpdateFirewallFirewallsRequestBody, error) {
	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("rules") {
		return nil, fmt.Errorf("at least one of --name or --rules is required")
	}

	attributes := &operations.UpdateFirewallFirewallsAttributes{}

	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		attributes.Name = &name
	}

	if cmd.Flags().Changed("rules") {
		rulesFlag, _ := cmd.Flags().GetString("rules")
		if strings.TrimSpace(rulesFlag) == "" {
			return nil, fmt.Errorf("--rules cannot be empty; pass [] to replace with no rules")
		}
		rules, err := parseUpdateRulesFlag(rulesFlag)
		if err != nil {
			return nil, err
		}
		attributes.Rules = rules
	}

	request := operations.UpdateFirewallFirewallsRequestBody{
		Data: operations.UpdateFirewallFirewallsData{
			Type:       operations.UpdateFirewallFirewallsTypeFirewalls,
			Attributes: attributes,
		},
	}

	return &request, nil
}

func (o *UpdateFirewallOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	request, err := buildUpdateFirewallRequest(cmd)
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

	response, err := client.Firewalls.Update(ctx, id, *request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Firewall != nil && response.Firewall.Data != nil && !lsh.Debug {
		firewall := Firewall{FirewallData: *response.Firewall.Data}
		utils.RenderStatic(firewall.GetData())
	}

	return nil
}
