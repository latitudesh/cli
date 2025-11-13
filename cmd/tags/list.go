package tags

import (
	"context"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListTagOperation{}
	cmd := &cobra.Command{
		Long:  "List all Tags in the team.\n",
		RunE:  op.run,
		Short: "List all Tags",
		Use:   "list",
	}

	return cmd
}

type ListTagOperation struct{}

func (o *ListTagOperation) run(cmd *cobra.Command, args []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := client.Tags.List(ctx)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	lsgTagData := []*Tag{}
	if response.CustomTags != nil && response.CustomTags.Data != nil {
		for _, tag := range response.CustomTags.Data {
			lshTag := Tag{
				Attributes: tag,
			}
			lsgTagData = append(lsgTagData, &lshTag)
		}
	}

	lshTags := Tags{
		Data: lsgTagData,
	}

	if !lsh.Debug {
		utils.Render(lshTags.GetData())
	}

	return nil
}
