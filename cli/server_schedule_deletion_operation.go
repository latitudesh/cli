package cli

import (
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationServersServerScheduleDeletionCmd() (*cobra.Command, error) {
	operation := ScheduleServerDeletionOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type ScheduleServerDeletionOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *ScheduleServerDeletionOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "schedule-deletion",
		Short:  "Schedule deletion",
		Long:   "Schedules the server to be removed at the end of the billing cycle.",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *ScheduleServerDeletionOperation) PromptAttributes(attributes interface{}) {
}

func (o *ScheduleServerDeletionOperation) registerFlags(cmd *cobra.Command) {
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

func (o *ScheduleServerDeletionOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *ScheduleServerDeletionOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewServerScheduleDeletionParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.ServerScheduleDeletion(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
