package cli

import (
	"github.com/latitudesh/lsh/client/ssh_keys"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationSSHKeysPutProjectSSHKeyCmd() (*cobra.Command, error) {
	operation := UpdateSSHKeyOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type UpdateSSHKeyOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *UpdateSSHKeyOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update an SSH key",
		Long:   `Allow you update SSH Key in a project. These keys can be used to access servers after deploy and reinstall actions.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *UpdateSSHKeyOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID from the SSH Key you want to update",
			Description: "ID from the SSH Key you want to update",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "project",
			Label:       "Project ID or Slug",
			Description: "Project ID or Slug",
			Required:    true,
		},
	}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name of the SSH Key",
			Description: "Name of the SSH Key",
			Required:    true,
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

func (o *UpdateSSHKeyOperation) preRun(cmd *cobra.Command, args []string) {
	projects := fetchUserProjects()
	o.PathParamFlags.AddFlagOption("project", projects)

	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *UpdateSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := ssh_keys.NewPutProjectSSHKeyParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)
	params.Body.Data.ID = params.SSHKeyID

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.SSHKeys.PutProjectSSHKey(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
