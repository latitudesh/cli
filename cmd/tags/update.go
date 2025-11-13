package tags

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
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
	client := lsh.NewClient()
	ctx := context.Background()

	pAttr := struct {
		ID string `json:"id"`
	}{}
	o.PathParamFlags.AssignValues(&pAttr)

	var name, description, slug, color string
	o.BodyAttributesFlags.AssignValues(&struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Slug        string `json:"slug"`
		Color       string `json:"color"`
	}{
		Name:        name,
		Description: description,
		Slug:        slug,
		Color:       color,
	})

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	// Create request
	updateTagType := operations.UpdateTagTagsTypeTags
	request := operations.UpdateTagTagsRequestBody{
		Data: &operations.UpdateTagTagsData{
			ID:   &pAttr.ID,
			Type: &updateTagType,
			Attributes: &operations.UpdateTagTagsAttributes{
				Name:        &name,
				Description: &description,
				Color:       &color,
			},
		},
	}

	// Call API
	response, err := client.Tags.Update(ctx, pAttr.ID, request)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	// Render response
	if response.CustomTag != nil && response.CustomTag.Data != nil {
		lshTag := Tag{
			Attributes: *response.CustomTag.Data,
		}

		if !lsh.Debug {
			utils.Render(lshTag.GetData())
		}
	}

	return nil
}
