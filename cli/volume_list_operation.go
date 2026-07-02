package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationVolumeListCmd() (*cobra.Command, error) {
	operation := VolumeListOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type VolumeListOperation struct {
	QueryParamFlags cmdflag.Flags
}

func (o *VolumeListOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all volume storages",
		Long:  "List all volume storages for your team, optionally filtered by project. The ATTACHED column shows whether each volume is attached to a host (and whether that host is this one).",
		RunE:  o.run,
		// --project is an optional filter here, not a required scope: by
		// default we list every volume in the team (server-side scoping
		// already limits results to the caller's team).
		Annotations: map[string]string{ProjectOptionalAnnotation: "true"},
		PreRun:      o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *VolumeListOperation) registerFlags(cmd *cobra.Command) {
	o.QueryParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	queryParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "project",
			Label:       "Project ID or Slug",
			Description: "Filter volume storages by project ID or slug",
			Required:    false,
		},
	}

	o.QueryParamFlags.Register(queryParamsSchema)
}

func (o *VolumeListOperation) preRun(cmd *cobra.Command, args []string) {
	o.QueryParamFlags.PreRun(cmd, args)
}

func (o *VolumeListOperation) run(cmd *cobra.Command, args []string) error {
	// Get optional project filter
	project, _ := cmd.Flags().GetString("project")

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	if viper.GetString("Authorization") == "" {
		return fmt.Errorf("API key not found. Please run 'lsh login' first")
	}

	client := lsh.NewClient()
	ctx := context.Background()

	// Create filter pointer if project is specified
	var filterProject *string
	if project != "" {
		filterProject = &project
	}

	response, err := client.BlockStorage.GetStorageVolumes(ctx, filterProject, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if lsh.Debug {
		return nil
	}

	var volumes []components.VolumeData
	if response.Object != nil {
		volumes = response.Object.Data
	}

	// Read the local host NQN once so we can flag volumes attached to THIS
	// host. Read-only: absent/unreadable simply disables the "this host" hint.
	localNQN := readLocalHostNQN()

	rows := make([]renderer.ResponseData, 0, len(volumes))
	for i := range volumes {
		rows = append(rows, newVolumeRow(volumes[i], localNQN))
	}

	utils.Render(rows)

	return nil
}

// VolumeRow is the flat, display-oriented projection of a volume used for both
// the table and the structured (-o json|yaml|csv) output, so the computed
// "attached" status is available in every format.
type VolumeRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SizeInGB int64  `json:"size_in_gb"`
	Project  string `json:"project,omitempty"`
	Attached string `json:"attached"`
}

func newVolumeRow(v components.VolumeData, localNQN string) *VolumeRow {
	row := &VolumeRow{Attached: "No"}

	if v.ID != nil {
		row.ID = *v.ID
	}

	if attr := v.Attributes; attr != nil {
		if attr.Name != nil {
			row.Name = *attr.Name
		}
		if attr.SizeInGb != nil {
			row.SizeInGB = *attr.SizeInGb
		}
		if attr.Project != nil {
			if attr.Project.Slug != nil && *attr.Project.Slug != "" {
				row.Project = *attr.Project.Slug
			} else if attr.Project.Name != nil {
				row.Project = *attr.Project.Name
			}
		}
		row.Attached = attachedStatus(attr.Initiators, localNQN)
	}

	return row
}

func (r *VolumeRow) TableRow() table.Row {
	return table.Row{
		"id":         table.Cell{Label: "ID", Value: table.String(r.ID)},
		"name":       table.Cell{Label: "Name", Value: table.String(r.Name)},
		"size_in_gb": table.Cell{Label: "Size (GB)", Value: table.String(fmt.Sprintf("%d", r.SizeInGB))},
		"project":    table.Cell{Label: "Project", Value: table.String(r.Project)},
		"attached":   table.Cell{Label: "Attached", Value: table.String(r.Attached)},
	}
}

// attachedStatus reports whether a volume is attached, based on the initiators
// (hosts) registered on it. When the local host NQN is known and matches one of
// them, we call it out so the operator can tell "attached to this box" apart
// from "attached somewhere else".
func attachedStatus(initiators []components.Initiators, localNQN string) string {
	if len(initiators) == 0 {
		return "No"
	}
	if localNQN != "" {
		for _, in := range initiators {
			if in.Nqn != nil && strings.EqualFold(strings.TrimSpace(*in.Nqn), localNQN) {
				return "Yes (this host)"
			}
		}
	}
	return "Yes"
}

// readLocalHostNQN returns the trimmed contents of /etc/nvme/hostnqn, or "" if
// the file is absent or unreadable. Unlike getHostNQN (used by attach), this is
// strictly read-only: it never generates an NQN or writes any file, so it is
// safe to call from a plain list command run without root.
func readLocalHostNQN() string {
	content, err := os.ReadFile("/etc/nvme/hostnqn")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
