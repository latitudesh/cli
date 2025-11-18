package tags

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	o := CreateTagOperation{}
	cmd := &cobra.Command{
		Long:  "Create a Tag in the team.\n",
		RunE:  o.run,
		Short: "Create a Tag",
		Use:   "create",
	}

	// Flags
	cmd.Flags().String("name", "", "Name of the Tag")
	cmd.Flags().String("description", "", "Description of the Tag")
	cmd.Flags().String("color", "", "Color of the Tag (hex code)")

	return cmd
}

type CreateTagOperation struct{}

func (o *CreateTagOperation) run(cmd *cobra.Command, args []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	noInput, _ := cmd.Flags().GetBool("no-input")

	var name, description, color string
	var err error

	if noInput {
		name, _ = cmd.Flags().GetString("name")
		description, _ = cmd.Flags().GetString("description")
		color, _ = cmd.Flags().GetString("color")

		if name == "" || color == "" {
			return fmt.Errorf("--name and --color are required in non-interactive mode")
		}
	} else {
		name, err = tui.RunTextInput("Tag Name", "Enter tag name...")
		if err != nil {
			return err
		}

		description, err = tui.RunTextInput("Description (optional)", "Enter description...")
		if err != nil {
			description = ""
		}

		colors := []string{
			"#FF0000 (Red)",
			"#00FF00 (Green)",
			"#0000FF (Blue)",
			"#FFC300 (Yellow)",
			"#9B59B6 (Purple)",
			"Custom...",
		}

		colorChoice, err := tui.RunList("Select Tag Color", colors, nil)
		if err != nil {
			return err
		}

		if colorChoice == "Custom..." {
			color, err = tui.RunTextInput("Custom Color", "#000000")
			if err != nil {
				return err
			}
		} else {
			color = colorChoice[:7]
		}
	}

	createTagType := operations.CreateTagTagsTypeTags
	request := operations.CreateTagTagsRequestBody{
		Data: &operations.CreateTagTagsData{
			Type: &createTagType,
			Attributes: &operations.CreateTagTagsAttributes{
				Name:        &name,
				Description: &description,
				Color:       &color,
			},
		},
	}

	response, err := client.Tags.Create(ctx, request)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.CustomTag != nil && response.CustomTag.Data != nil {
		fmt.Println(tui.SuccessStyle.Render("✓ Tag created successfully!"))

		if !lsh.Debug {
			lshTag := Tag{
				Attributes: *response.CustomTag.Data,
			}
			utils.Render(lshTag.GetData())
		}
	}

	return nil
}
