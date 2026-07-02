package cli

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/api/resource"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"

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
		Use:   "create",
		Short: "Deploy a bare metal server",
		// MANUAL — keep when regenerating
		Example: `  # Minimal deploy
  lsh servers create \
    --project=my-project \
    --site=NYC2 \
    --plan=c2-small-x86 \
    --operating_system=ubuntu_22_04 \
    --hostname=web-01

  # With SSH keys
  lsh servers create \
    --project=my-project \
    --site=NYC2 \
    --plan=c2-small-x86 \
    --hostname=web-02 \
    --ssh_keys=key_abc,key_def`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)
	wait.AddFlags(cmd)

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
			Description: "The operating system slug for the new server (e.g. ubuntu_22_04_x64_lts).",
			Required:    true,
			Options:     server.SupportedOperatingSystems,
		},
		&cmdflag.String{
			Name:        "plan",
			Label:       "Plan",
			Description: `The plan slug to provision (e.g. c3-small-x86). Run "lsh plans list" to see what is available.`,
			Required:    true,
			Options:     server.SupportedPlans,
		},
		&cmdflag.String{
			Name:        "site",
			Label:       "Site",
			Description: "The site slug to deploy the server in (e.g. NYC2).",
			Required:    true,
			Options:     server.SupportedSites,
		},
		&cmdflag.String{
			Name:        "billing",
			Label:       "Billing Type",
			Description: "The server billing type. Accepts 'hourly', 'monthly' or 'yearly'.",
			Required:    false,
			Options:     server.SupportedBillingTypes,
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
			Description: "RAID mode for the server (e.g. raid-0, raid-1).",
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
		&cmdflag.String{
			Name:        "project",
			Label:       "Project",
			Description: "The project (ID or Slug) to deploy the server",
			Required:    true,
		},
	}

	o.BodyAttributesFlags.Register(schema)
}

func (o *CreateServerOperation) preRun(cmd *cobra.Command, args []string) {
	projects := fetchUserProjects()
	o.BodyAttributesFlags.AddFlagOption("project", projects)

	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateServerOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := servers.NewCreateServerParams()
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.Servers.CreateServer(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	waitOpts := wait.OptionsFrom(cmd)

	// When waiting, the create payload is an optimistic snapshot (it can report
	// "on" while the server is still deploying); skip it and let the wait render
	// the real final state instead.
	if !lsh.Debug && !waitOpts.Enabled {
		utils.Render(response.GetData())
	}

	var serverID string
	if p := response.GetPayload(); p != nil && p.Data != nil {
		serverID = p.Data.ID
	}
	want, fail := serverProvisionTargets()
	return waitForServerState(cmd, serverID, want, fail)
}

func fetchUserProjects() []string {
	userProjects := []string{}
	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.Projects.List(ctx, operations.GetProjectsRequest{})
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if response.Projects != nil && response.Projects.Data != nil {
		for _, proj := range response.Projects.Data {
			if proj.Attributes != nil && proj.Attributes.Name != nil {
				userProjects = append(userProjects, *proj.Attributes.Name)
			}
			if proj.ID != nil {
				userProjects = append(userProjects, *proj.ID)
			}
		}
	}

	return userProjects
}
