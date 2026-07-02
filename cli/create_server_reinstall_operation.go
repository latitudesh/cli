package cli

import (
	"fmt"

	"github.com/latitudesh/lsh/client/server_reinstall"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/api/resource"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"

	"github.com/go-openapi/swag"
	"github.com/spf13/cobra"
)

func makeOperationServerReinstallCmd() (*cobra.Command, error) {
	operation := CreateServerReinstallOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type CreateServerReinstallOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateServerReinstallOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "reinstall",
		Short: "Reinstall a server",
		// MANUAL — keep when regenerating
		Example: `  lsh servers reinstall --id sv_xxxxxxxx --operating_system=ubuntu_22_04_x64_lts`,
		Long:    "Submit a reinstall request to a server.",
		RunE:    o.run,
		PreRun:  o.preRun,
	}

	o.registerFlags(cmd)
	wait.AddFlags(cmd)

	return cmd, nil
}

func (o *CreateServerReinstallOperation) registerFlags(cmd *cobra.Command) {
	server := resource.NewServerResource()
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Server ID",
			Description: "The Server Id (Required).",
			Required:    true,
		},
	}

	bodyAttributesFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "operating_system",
			Label:       "Operating System",
			Description: "The operating system slug for the reinstall (e.g. ubuntu_22_04_x64_lts).",
			Required:    false,
			Options:     server.SupportedOperatingSystems,
		},
		&cmdflag.String{
			Name:        "hostname",
			Label:       "",
			Description: "The server hostname",
			Required:    false,
		},
		&cmdflag.StringSlice{
			Name:        "ssh_keys",
			Label:       "SSH Keys",
			Description: "The SSH Keys to set on the server",
			Required:    false,
		},
		&cmdflag.Int64{
			Name:        "user_data",
			Label:       "User Data",
			Description: "User data to set on the server",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "raid",
			Label:       "RAID Level",
			Description: "RAID mode for the server (e.g. raid-0, raid-1).",
			Required:    false,
			Options:     server.SupportedRAIDLevels,
		},
		&cmdflag.String{
			Name:        "ipxe_url",
			Label:       "iPXE URL",
			Description: "URL where iPXE script is stored on, necessary for custom image deployments.This attribute is required when iPXE is selected as operating system.",
			Required:    false,
		},
	}

	o.BodyAttributesFlags.Register(bodyAttributesFlagsSchema)
	o.PathParamFlags.Register(pathParamsFlagsSchema)

}

func (o *CreateServerReinstallOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateServerReinstallOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := server_reinstall.NewCreateServerReinstallParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if swag.IsZero(*params.Body.Data.Attributes) {
		fmt.Println("Skipped action: no params provided")
		return nil
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.ServerReinstall.CreateServerReinstall(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	waitEnabled := wait.OptionsFrom(cmd).Enabled
	if !lsh.Debug && !waitEnabled {
		response.Render()
	}

	serverID, _ := cmd.Flags().GetString("id")
	want, fail := serverProvisionTargets()
	return waitForServerState(cmd, serverID, want, fail)
}
