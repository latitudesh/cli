package cli

import (
	"context"
	"fmt"
	"os"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationBlockCreateCmd() (*cobra.Command, error) {
	operation := BlockCreateOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type BlockCreateOperation struct {
	BodyAttributesFlags cmdflag.Flags
}

func (o *BlockCreateOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Create a new block storage",
		Long:   "Create a new block storage with specified size and location",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *BlockCreateOperation) registerFlags(cmd *cobra.Command) {
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	bodyAttributesSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "project",
			Label:       "Project ID or Slug",
			Description: "The project to create the block storage in",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "plan",
			Label:       "Plan ID or Slug",
			Description: "The storage plan to use",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "location",
			Label:       "Location",
			Description: "The location/site for the block storage",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "description",
			Label:       "Description",
			Description: "Optional description for the block storage",
			Required:    false,
		},
	}

	o.BodyAttributesFlags.Register(bodyAttributesSchema)
}

func (o *BlockCreateOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *BlockCreateOperation) run(cmd *cobra.Command, args []string) error {
	// Get required flags
	project, _ := cmd.Flags().GetString("project")
	plan, _ := cmd.Flags().GetString("plan")
	location, _ := cmd.Flags().GetString("location")
	description, _ := cmd.Flags().GetString("description")

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	// Initialize the SDK client
	apiKey := viper.GetString("Authorization")
	if apiKey == "" {
		return fmt.Errorf("API key not found. Please run 'lsh login' first")
	}

	ctx := context.Background()
	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity(apiKey),
	)

	fmt.Fprintf(os.Stdout, "Creating block storage...\n")
	fmt.Fprintf(os.Stdout, "  Project: %s\n", project)
	fmt.Fprintf(os.Stdout, "  Plan: %s\n", plan)
	fmt.Fprintf(os.Stdout, "  Location: %s\n", location)
	if description != "" {
		fmt.Fprintf(os.Stdout, "  Description: %s\n", description)
	}
	fmt.Fprintf(os.Stdout, "\n⚠️  Note: Block storage creation via CLI is coming soon.\n")
	fmt.Fprintf(os.Stdout, "The SDK method exists but needs proper request body mapping.\n")
	fmt.Fprintf(os.Stdout, "For now, create block storage via the web dashboard at https://www.latitude.sh\n")

	_ = client
	_ = ctx

	return nil
}
