package cli

import (
	"github.com/latitudesh/lsh/client/ssh_keys"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationSSHKeysPostProjectSSHKeyCmd() (*cobra.Command, error) {
	operation := CreateSSHKeyOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type CreateSSHKeyOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateSSHKeyOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Create an SSH key",
		Long:   `Allow you create SSH Keys in a project. These keys can be used to access servers after deploy and reinstall actions.`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *CreateSSHKeyOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "project",
			Label:       "Project ID or Slug",
			Description: "Project ID or Slug",
			Required:    true,
		}}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name of the SSH Key",
			Description: "Name of the SSH Key",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "public_key",
			Label:       "SSH Public Key",
			Description: "SSH Public Key",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.BodyAttributesFlags.Register(bodyFlagsSchema)
}

func (o *CreateSSHKeyOperation) preRun(cmd *cobra.Command, args []string) {
	projects := fetchUserProjects()
	o.PathParamFlags.AddFlagOption("project", projects)

	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := ssh_keys.NewPostProjectSSHKeyParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.SSHKeys.PostProjectSSHKey(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
