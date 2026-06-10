package cli

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
)

func makeOperationGroupOperatingSystemsCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     "operating-systems",
		Aliases: []string{"os"},
		Short:   "List operating systems available for deployment",
	}

	listCmd, err := makeOperationOperatingSystemsListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	return cmd, nil
}

func makeOperationOperatingSystemsListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List operating systems that can be installed on Latitude servers",
		Example:      `  lsh operating-systems list`,
		Args:         cobra.NoArgs,
		RunE:         runOperatingSystemsList,
		SilenceUsage: true,
	}
	return cmd, nil
}

type operatingSystemRow struct {
	ID      string `json:"id,omitempty"`
	Slug    string `json:"slug,omitempty"`
	Name    string `json:"name,omitempty"`
	Distro  string `json:"distro,omitempty"`
	Version string `json:"version,omitempty"`
	User    string `json:"user,omitempty"`
}

func (o operatingSystemRow) TableRow() table.Row {
	return table.Row{
		"id":      {Value: o.ID, Label: "ID"},
		"slug":    {Value: o.Slug, Label: "Slug"},
		"name":    {Value: o.Name, Label: "Name"},
		"distro":  {Value: o.Distro, Label: "Distro"},
		"version": {Value: o.Version, Label: "Version"},
		"user":    {Value: o.User, Label: "Default User"},
	}
}

func runOperatingSystemsList(_ *cobra.Command, _ []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.OperatingSystems.ListPlans(ctx, nil, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip rendering.")
		return nil
	}

	if resp == nil || resp.OperatingSystems == nil {
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.OperatingSystems.Data))
	for i := range resp.OperatingSystems.Data {
		rows = append(rows, operatingSystemToRow(&resp.OperatingSystems.Data[i]))
	}
	renderer.Render(rows)
	return nil
}

func operatingSystemToRow(o *components.OperatingSystemData) operatingSystemRow {
	row := operatingSystemRow{}
	if o == nil {
		return row
	}
	if o.ID != nil {
		row.ID = *o.ID
	}
	if o.Attributes == nil {
		return row
	}
	if o.Attributes.Slug != nil {
		row.Slug = *o.Attributes.Slug
	}
	if o.Attributes.Name != nil {
		row.Name = *o.Attributes.Name
	}
	if o.Attributes.Distro != nil {
		row.Distro = *o.Attributes.Distro
	}
	if o.Attributes.Version != nil {
		row.Version = *o.Attributes.Version
	}
	if o.Attributes.User != nil {
		row.User = *o.Attributes.User
	}
	return row
}
