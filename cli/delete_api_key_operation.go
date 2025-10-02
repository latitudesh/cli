package cli

import (
	"github.com/latitudesh/lsh/client/api_keys"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationAPIKeysDeleteAPIKeyCmd() (*cobra.Command, error) {
	operation := DeleteAPIKeyOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type DeleteAPIKeyOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *DeleteAPIKeyOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Delete an API Key",
		Long:   `Delete an existing API Key. Once deleted, the API Key can no longer be used to access the API.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *DeleteAPIKeyOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the API Key you want to delete",
			Description: "ID",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DeleteAPIKeyOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DeleteAPIKeyOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := api_keys.NewDeleteAPIKeyParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.APIKeys.DeleteAPIKey(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
