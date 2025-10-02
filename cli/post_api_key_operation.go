package cli

import (
	"github.com/latitudesh/lsh/client/api_keys"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationAPIKeysPostAPIKeyCmd() (*cobra.Command, error) {
	operation := CreateAPIKeyOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type CreateAPIKeyOperation struct {
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateAPIKeyOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Create an API Key",
		Long:   `Create a new API Key that is tied to the current user account. The created API key is only listed ONCE upon creation. It can however be regenerated or deleted.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *CreateAPIKeyOperation) registerFlags(cmd *cobra.Command) {
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name of the API Key",
			Description: "Name of the API Key",
			Required:    true,
		},
	}

	o.BodyAttributesFlags.Register(schema)
}

func (o *CreateAPIKeyOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateAPIKeyOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := api_keys.NewPostAPIKeyParams()
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.APIKeys.PostAPIKey(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
