package firewalls

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateFirewallOperation{}
	cmd := &cobra.Command{
		Long: "Create a firewall in the given project.\n\n" +
			"Rules can be supplied from a JSON file with --rules @path/to/rules.json.\n" +
			"The file must contain a JSON array of rule objects, e.g.:\n\n" +
			`  [{"from":"ANY","to":"ANY","protocol":"TCP","port":"22","description":"ssh"}]`,
		RunE:  op.run,
		Short: "Create a firewall",
		Example: `  lsh firewalls create --name web --project my-project --rules @rules.json
  lsh firewalls create --name web --project my-project`,
		Use: "create",
	}

	cmd.Flags().String("name", "", "Name of the firewall")
	cmd.Flags().String("project", "", "Project ID or slug the firewall belongs to")
	cmd.Flags().String("rules", "", "Firewall rules as a JSON array, or @path to a JSON file")

	return cmd
}

type CreateFirewallOperation struct{}

// parseRulesFlag resolves the --rules flag value into SDK rule structs. A value
// prefixed with '@' is read from a file; otherwise the value itself is treated
// as inline JSON. An empty value yields no rules.
func parseRulesFlag(value string) ([]operations.CreateFirewallRules, error) {
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

	var rules []operations.CreateFirewallRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("could not parse firewall rules JSON: %w", err)
	}

	return rules, nil
}

func (o *CreateFirewallOperation) run(cmd *cobra.Command, args []string) error {
	request, err := buildCreateFirewallRequest(cmd)
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

	response, err := client.Firewalls.Create(ctx, *request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Firewall != nil && response.Firewall.Data != nil {
		if !lsh.Debug {
			fmt.Println(tui.SuccessStyle.Render("✓ Firewall created successfully!"))
			firewall := Firewall{FirewallData: *response.Firewall.Data}
			utils.RenderStatic(firewall.GetData())
		}
	}

	return nil
}

// buildCreateFirewallRequest assembles the SDK request from the command flags.
// It is split out from run so tests can exercise flag/rules parsing without a
// network call.
func buildCreateFirewallRequest(cmd *cobra.Command) (*operations.CreateFirewallFirewallsRequestBody, error) {
	name, _ := cmd.Flags().GetString("name")
	project, _ := cmd.Flags().GetString("project")
	rulesFlag, _ := cmd.Flags().GetString("rules")

	if name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if project == "" {
		return nil, fmt.Errorf("--project is required")
	}

	rules, err := parseRulesFlag(rulesFlag)
	if err != nil {
		return nil, err
	}

	request := operations.CreateFirewallFirewallsRequestBody{
		Data: operations.CreateFirewallData{
			Type: operations.CreateFirewallTypeFirewalls,
			Attributes: &operations.CreateFirewallAttributes{
				Name:    name,
				Project: project,
				Rules:   rules,
			},
		},
	}

	return &request, nil
}
