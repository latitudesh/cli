package cli

import (
	"fmt"

	"github.com/go-openapi/swag"
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/api/resource"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationServersUpdateServerCmd() (*cobra.Command, error) {
	operation := UpdateServerOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type UpdateServerOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *UpdateServerOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update a server",
		Long:   "Update server information.",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *UpdateServerOperation) registerFlags(cmd *cobra.Command) {
	server := resource.NewServerResource()

	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the Server you want to update",
			Description: "Server ID",
			Required:    true,
		},
	}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "hostname",
			Label:       "Hostname",
			Description: "Hostname",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "billing",
			Label:       "Billing",
			Description: "Billing",
			Options:     server.SupportedBillingTypes,
			Required:    false,
		},
		&cmdflag.StringSlice{
			Name:        "tags",
			Label:       "Tags",
			Description: "Tags",
			Required:    false,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.BodyAttributesFlags.Register(bodyFlagsSchema)
}

func (o *UpdateServerOperation) PromptQueryParams(params interface{}) {
}

func (o *UpdateServerOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *UpdateServerOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewUpdateServerParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)
	params.Body.Data.ID = params.ID

	if swag.IsZero(*params.Body.Data.Attributes) {
		fmt.Println("Skipped action: no params provided")
		return nil
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.UpdateServer(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
