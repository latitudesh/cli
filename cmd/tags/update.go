package tags

import (
	sdk "github.com/latitudesh/latitudesh-go"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	o := UpdateTagOperation{}
	cmd := &cobra.Command{
		Long:   "Update a Tag in the team.\n",
		RunE:   o.run,
		PreRun: o.preRun,
		Short:  "Update Tag",
		Use:    "update",
	}
	o.registerFlags(cmd)

	return cmd
}

type UpdateTagOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *UpdateTagOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "ID",
			Description: "ID of the Tag",
			Required:    true,
		},
	}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "name",
			Label:       "Name",
			Description: "Name of the Tag",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "description",
			Label:       "Description",
			Description: "Description of the Tag",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "slug",
			Label:       "Slug",
			Description: "Slug of the Tag",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "color",
			Label:       "Color",
			Description: "Color of the Tag",
			Required:    false,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.BodyAttributesFlags.Register(bodyFlagsSchema)
}

func (o *UpdateTagOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *UpdateTagOperation) run(cmd *cobra.Command, args []string) error {
	c := lsh.NewClient()

	pAttr := struct {
		ID string `json:"id"`
	}{}
	o.PathParamFlags.AssignValues(&pAttr)

	bAttr := sdk.TagUpdateAttributes{}
	o.BodyAttributesFlags.AssignValues(&bAttr)

	request := &sdk.TagUpdateRequest{
		Data: sdk.TagUpdateData{
			ID:         pAttr.ID,
			Type:       "Tag",
			Attributes: bAttr,
		},
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	tag, _, err := c.Tags.Update(pAttr.ID, request)
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
