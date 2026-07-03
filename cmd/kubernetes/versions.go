package kubernetes

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

// NewVersionsCmd returns the `kubernetes versions` command group.
func NewVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "versions",
		Short:   "Manage Kubernetes versions",
		Long:    "Inspect the Kubernetes versions available for new clusters.\n",
		Example: `  lsh kubernetes versions list`,
	}
	cmd.AddCommand(newVersionsListCmd())
	return cmd
}

func newVersionsListCmd() *cobra.Command {
	op := ListVersionsOperation{}
	cmd := &cobra.Command{
		Long:    "List the Kubernetes versions available for new clusters.\n",
		RunE:    op.run,
		Short:   "List available Kubernetes versions",
		Example: `  lsh kubernetes versions list -o json`,
		Use:     "list",
		Aliases: []string{"ls"},
	}
	return cmd
}

type ListVersionsOperation struct{}

// Version wraps an available Kubernetes version for rendering.
type Version struct {
	components.KubernetesAvailableVersionsData
}

func (v *Version) TableRow() table.Row {
	return table.Row{
		"minor":  table.Cell{Label: "Minor", Value: table.String(getStr(v.Minor))},
		"latest": table.Cell{Label: "Latest", Value: table.String(getStr(v.Latest))},
	}
}

func (o *ListVersionsOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.KubernetesClusters.ListKubernetesAvailableVersions(ctx, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	var data []renderer.ResponseData
	if response.KubernetesAvailableVersions != nil {
		versions := response.KubernetesAvailableVersions.Data
		for i := range versions {
			data = append(data, &Version{KubernetesAvailableVersionsData: versions[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(data)
	}

	return nil
}
