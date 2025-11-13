package tags

import (
	"context"
	"fmt"
	"net/http"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewDestroyCmd() *cobra.Command {
	op := DestroyTagOperation{}
	cmd := &cobra.Command{
		Long:   "Delete a Tag in the team.\n",
		RunE:   op.run,
		PreRun: op.preRun,
		Short:  "Delete Tag",
		Use:    "destroy",
	}
	op.registerFlags(cmd)

	return cmd
}

type DestroyTagOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *DestroyTagOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the Tag you want to delete",
			Description: "The Tag ID",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DestroyTagOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DestroyTagOperation) run(cmd *cobra.Command, args []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	attr := struct {
		ID string `json:"id"`
	}{}

	o.PathParamFlags.AssignValues(&attr)

	resp, err := client.Tags.Delete(ctx, attr.ID)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		// Check status code via HTTPMeta.Response.StatusCode
		if resp.HTTPMeta.Response != nil && resp.HTTPMeta.Response.StatusCode != http.StatusNoContent {
			fmt.Printf("Warning: Unexpected status code: %d\n", resp.HTTPMeta.Response.StatusCode)
		}
		fmt.Printf("\n\nTag deleted successfully!\n\n")
	}
	return nil
}
