package cli

import (
	"github.com/latitudesh/lsh/client/api_keys"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationAPIKeysUpdateAPIKeyCmd() (*cobra.Command, error) {
	operation := UpdateAPIKeyOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type UpdateAPIKeyOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *UpdateAPIKeyOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update an API Key",
		Long:   `Regenerate an existing API Key that is tied to the current user. This overrides the previous key.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *UpdateAPIKeyOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the API Key you want to update",
			Description: "ID",
			Required:    true,
		},
	}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name of the API Key",
			Description: "Name of the API Key",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.BodyAttributesFlags.Register(bodyFlagsSchema)
}

func (o *UpdateAPIKeyOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *UpdateAPIKeyOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := api_keys.NewUpdateAPIKeyParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)
	params.Body.Data.ID = params.ID

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.APIKeys.UpdateAPIKey(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
