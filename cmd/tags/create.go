package tags

import (
	sdk "github.com/latitudesh/latitudesh-go"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	o := CreateTagOperation{}
	cmd := &cobra.Command{
		Long:   "Create a Tag in the team.\n",
		RunE:   o.run,
		PreRun: o.preRun,
		Short:  "Create a Tag",
		Use:    "create",
	}
	o.registerFlags(cmd)

	return cmd
}

type CreateTagOperation struct {
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateTagOperation) registerFlags(cmd *cobra.Command) {
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name",
			Description: "Name of the Tag",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "description",
			Label:       "Description",
			Description: "Description of the Tag",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "color",
			Label:       "Color",
			Description: "Color of the Tag",
			Required:    true,
		},
	}

	o.BodyAttributesFlags.Register(schema)
}

func (o *CreateTagOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateTagOperation) run(cmd *cobra.Command, args []string) error {
	c := lsh.NewClient()

	attr := sdk.TagCreateAttributes{}
	o.BodyAttributesFlags.AssignValues(&attr)

	createRequest := &sdk.TagCreateRequest{
		Data: sdk.TagCreateData{
			Type:       "Tag",
			Attributes: attr,
		},
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	tag, _, err := c.Tags.Create(createRequest)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	lshTag := Tag{
		Attributes: *tag,
	}

	if !lsh.Debug {
		utils.Render(lshTag.GetData())
	}

	return nil
}
