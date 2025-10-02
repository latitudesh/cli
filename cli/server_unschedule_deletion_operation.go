package cli

import (
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationServersServerUnscheduleDeletionCmd() (*cobra.Command, error) {
	operation := UnscheduleServerDeletionOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type UnscheduleServerDeletionOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *UnscheduleServerDeletionOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "unschedule-deletion",
		Short:  "Unschedule deletion",
		Long:   "Unschedules the server removal at the end of the billing cycle.",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *UnscheduleServerDeletionOperation) PromptAttributes(attributes interface{}) {
}

func (o *UnscheduleServerDeletionOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Server ID",
			Description: "Server ID",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *UnscheduleServerDeletionOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *UnscheduleServerDeletionOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewServerUnscheduleDeletionParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.ServerUnscheduleDeletion(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
