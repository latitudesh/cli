package tags

import (
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
	c := lsh.NewClient()

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	tags, _, err := c.Tags.List(nil)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	lsgTagData := []*Tag{}
	for _, tag := range tags {
		lshTag := Tag{
			Attributes: tag,
		}
		lsgTagData = append(lsgTagData, &lshTag)
	}

	lshTags := Tags{
		Data: lsgTagData,
	}

	if !lsh.Debug {
		utils.Render(lshTags.GetData())
	}

	return nil
}
