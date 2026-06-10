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

func makeOperationGroupRegionsCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "regions",
		Short: "List Latitude regions",
	}

	listCmd, err := makeOperationRegionsListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	return cmd, nil
}

func makeOperationRegionsListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List every Latitude region available to your account",
		Example:      `  lsh regions list`,
		Args:         cobra.NoArgs,
		RunE:         runRegionsList,
		SilenceUsage: true,
	}
	return cmd, nil
}

// regionRow is the renderer-friendly projection of a region. It carries
// both JSON tags (used when `-o json` is set) and a TableRow method
// (used for the default table / interactive output).
type regionRow struct {
	ID          string `json:"id,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Name        string `json:"name,omitempty"`
	CountrySlug string `json:"country_slug,omitempty"`
	CountryName string `json:"country_name,omitempty"`
}

func (r regionRow) TableRow() table.Row {
	return table.Row{
		"id":      {Value: r.ID, Label: "ID"},
		"slug":    {Value: r.Slug, Label: "Slug"},
		"name":    {Value: r.Name, Label: "Name"},
		"country": {Value: r.CountryName, Label: "Country"},
	}
}

func runRegionsList(_ *cobra.Command, _ []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.Regions.Get(ctx, nil, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.Regions == nil {
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.Regions.Data))
	for resp != nil && resp.Regions != nil {
		for i := range resp.Regions.Data {
			rows = append(rows, regionsDataToRow(&resp.Regions.Data[i]))
		}
		if resp.Next == nil {
			break
		}
		resp, err = resp.Next()
		if err != nil {
			utils.PrintError(err)
			return nil
		}
	}
	renderer.Render(rows)
	return nil
}

func regionsDataToRow(r *components.RegionsData) regionRow {
	row := regionRow{}
	if r == nil {
		return row
	}
	if r.ID != nil {
		row.ID = *r.ID
	}
	if r.Attributes == nil {
		return row
	}
	if r.Attributes.Slug != nil {
		row.Slug = *r.Attributes.Slug
	}
	if r.Attributes.Name != nil {
		row.Name = *r.Attributes.Name
	}
	if r.Attributes.Country != nil {
		if r.Attributes.Country.Slug != nil {
			row.CountrySlug = *r.Attributes.Country.Slug
		}
		if r.Attributes.Country.Name != nil {
			row.CountryName = *r.Attributes.Country.Name
		}
	}
	return row
}
