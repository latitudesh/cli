package cli

import (
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationServersDestroyServerCmd() (*cobra.Command, error) {
	operation := DestroyServerOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type DestroyServerOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *DestroyServerOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Delete a server",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *DestroyServerOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the Server you want to delete",
			Description: "Server ID",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DestroyServerOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DestroyServerOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewDestroyServerParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.DestroyServer(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
