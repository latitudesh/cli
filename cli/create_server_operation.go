package cli

import (
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/internal/api/resource"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationServersCreateServerCmd() (*cobra.Command, error) {
	operation := CreateServerOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type CreateServerOperation struct {
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateServerOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Deploy a new server.",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *CreateServerOperation) registerFlags(cmd *cobra.Command) {
	server := resource.NewServerResource()

	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "hostname",
			Label:       "Hostname",
			Description: "The server hostname",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "operating_system",
			Label:       "Operating System",
			Description: `Enum: ["ipxe","windows_server_2019_std_v1","ubuntu_22_04_x64_lts","debian_11","rockylinux_8","debian_10","rhel8","centos_7_4_x64","centos_8_x64","ubuntu_20_04_x64_lts","debian_12","ubuntu22_ml_in_a_box","windows2022"]. The operating system for the new server`,
			Required:    true,
			Options:     server.SupportedOperatingSystems,
		},
		&cmdflag.String{
			Name:        "plan",
			Label:       "Plan",
			Description: `Enum: ["c2-large-x86","c2-medium-x86","c2-small-x86","c3-large-x86","c3-medium-x86","c3-small-x86","c3-xlarge-x86","g3-large-x86","g3-medium-x86","g3-small-x86","g3-xlarge-x86","m3-large-x86","s2-small-x86","s3-large-x86"]. The plan to choose server from`,
			Required:    true,
			Options:     server.SupportedPlans,
		},
		&cmdflag.String{
			Name:        "project",
			Label:       "Project",
			Description: "The project (ID or Slug) to deploy the server",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "site",
			Label:       "Site",
			Description: `Enum: ["ASH","BGT","BUE","CHI","DAL","FRA","LAX","LON","MEX","MEX2","MIA","MIA2","NYC","SAN","SAN2","SAO","SAO2","SYD","TYO","TYO2"]. The site to deploy the server`,
			Required:    true,
			Options:     server.SupportedSites,
		},
		&cmdflag.String{
			Name:        "billing",
			Label:       "Billing Type",
			Description: "The server billing type. Accepts 'hourly', 'monthly' or 'yearly'.",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "ipxe_url",
			Label:       "iPXE URL",
			Description: "URL where iPXE script is stored on, necessary for custom image deployments. This attribute is required when iPXE is selected as operating system.",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "raid",
			Label:       "RAID Level",
			Description: `Enum: ["raid-0","raid-1"]. RAID mode for the server`,
			Required:    false,
			Options:     server.SupportedRAIDLevels,
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
	}

	o.BodyAttributesFlags.Register(schema)
}

func (o *CreateServerOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateServerOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewCreateServerParams()
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if dryRun {
		logDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.CreateServer(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !debug {
		utils.Render(response.GetData())
	}

	return nil
}
