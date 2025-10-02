package cli

import (
	"github.com/latitudesh/lsh/client/projects"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

type DeleteProjectOperation struct {
	PathParamFlags cmdflag.Flags
}

func makeOperationProjectsDeleteProjectCmd() (*cobra.Command, error) {
	operation := DeleteProjectOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (o *DeleteProjectOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Delete a project",
		Long:   `Deletes a project from the current team.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *DeleteProjectOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id_or_slug",
			Label:       "ID or Slug from the Project you want to delete",
			Description: "The project ID or Slug",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DeleteProjectOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DeleteProjectOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := projects.NewDeleteProjectParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Projects.DeleteProject(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
