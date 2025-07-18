package cli

import (
	"fmt"

	"github.com/latitudesh/lsh/client/server_reinstall"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/api/resource"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

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
		Use:    "reinstall",
		Short:  "Reintall a server",
		Long:   "Submit a reinstall request to a server.",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

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
			Description: `Enum: ["ipxe","windows_server_2019_std_v1","ubuntu_22_04_x64_lts","debian_11","rockylinux_8","debian_10","rhel8","centos_7_4_x64","centos_8_x64","ubuntu_20_04_x64_lts","debian_12","ubuntu22_ml_in_a_box","windows2022"]. The operating system for the new server`,
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
			Description: `Enum: ["raid-0","raid-1"]. RAID mode for the server`,
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

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
