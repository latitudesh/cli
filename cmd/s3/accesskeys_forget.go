package s3

import (
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysForgetCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "forget <name>",
		Aliases: []string{"unsave"},
		Short:   "Remove a saved access key from this profile (the key stays valid on the API)",
		Long: `Remove an access key from the active profile only. The key keeps working for
whoever else has it; use 'lsh s3 access-keys delete' to revoke it on the API.`,
		Example: `  lsh s3 access-keys forget ci-deploy
  lsh s3 access-keys forget ci-deploy --profile acme`,
		Args: cobra.ExactArgs(1),
		RunE: runAccessKeysForget,
	})
	return cmd
}

// forgetResult is the structured output of forget.
type forgetResult struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
	Removed bool   `json:"removed"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

func (f forgetResult) TableRow() table.Row {
	status := "removed"
	if f.DryRun {
		status = "dryrun"
	}
	return table.Row{
		"name":    {Label: "Name", Value: f.Name},
		"profile": {Label: "Profile", Value: f.Profile},
		"status":  {Label: "Status", Value: status},
	}
}

func runAccessKeysForget(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	_, profileName, p, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return printErr(err)
	}
	if _, ok := p.ObjectStorageKeys()[name]; !ok {
		return printErr(exitcode.Errorf(exitcode.NotFound, "no saved access key named %q in profile %s; run 'lsh s3 access-keys list --saved'", name, profileName))
	}
	res := forgetResult{Name: name, Profile: profileName, Removed: true, DryRun: dryRun()}
	if !dryRun() {
		if _, _, err := objectstorage.ForgetKey(profileFlag(cmd), name); err != nil {
			return printErr(objectstorage.Humanize(err, nil, nil))
		}
	}
	if isHuman() {
		prefix := ""
		if res.DryRun {
			prefix = "(dryrun) "
		}
		fmt.Printf("%sforget: access key %q removed from profile %s\n", prefix, name, profileName)
		return nil
	}
	render([]renderer.ResponseData{res})
	return nil
}
