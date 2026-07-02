package kubernetes

import (
	cobra "github.com/spf13/cobra"
)

// NewClustersCmd returns the `kubernetes clusters` command group.
func NewClustersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clusters",
		Short: "Manage Kubernetes clusters",
		Long:  "Create, inspect, list, and delete the team's Kubernetes clusters.\n",
		Example: `  lsh kubernetes clusters list --project my-project
  lsh kubernetes clusters create --project my-project --region SAO2 --plan c2-small-x86 --wait
  lsh kubernetes clusters get kc_xxxxxxxx
  lsh kubernetes clusters delete kc_xxxxxxxx`,
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewGetCmd())
	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(NewDeleteCmd())

	return cmd
}
