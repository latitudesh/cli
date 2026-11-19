package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysDeleteCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "delete <name|access-key-id|username>",
		Aliases: []string{"rm", "revoke"},
		Short:   "Delete an access key on the API and forget it locally",
		Long: `Delete an access key. The key is looked up by name, access key ID or username
in the API (every project unless --project is given) and among the saved keys.
When several keys match (same name in two classes, sites or projects) the
command lists them and asks for --storage-class, --region or --project.

The key is revoked on the API and removed from the active profile. Anything
still using it stops working immediately.`,
		Example: `  lsh s3 access-keys delete ci-deploy
  lsh s3 access-keys delete XL68DDURVGUUOULWPCAE --yes
  lsh s3 access-keys delete backup --storage-class high_performance --region TYO4 --project my-project`,
		Args: cobra.ExactArgs(1),
		RunE: runAccessKeysDelete,
	})
	addProjectFlag(cmd, true, "project of the key (ID or slug); every project is searched when omitted")
	cmd.Flags().StringP("storage-class", "c", "", "disambiguate by storage class: standard or high_performance")
	cmd.Flags().String("region", "", "disambiguate by site (e.g. TYO4); required for high_performance keys when unknown")
	addYesFlag(cmd)
	return cmd
}

// deleteResult is the structured output of delete.
type deleteKeyResult struct {
	Name         string   `json:"name"`
	AccessKeyID  string   `json:"access_key_id"`
	Username     string   `json:"username,omitempty"`
	StorageClass string   `json:"storage_class"`
	Site         string   `json:"site,omitempty"`
	Project      string   `json:"project"`
	Deleted      bool     `json:"deleted"`
	Forgotten    []string `json:"forgotten,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
}

func (d deleteKeyResult) TableRow() table.Row {
	status := "deleted"
	if d.DryRun {
		status = "dryrun"
	}
	return table.Row{
		"name":          {Label: "Name", Value: d.Name},
		"access_key_id": {Label: "Access Key ID", Value: d.AccessKeyID},
		"storage_class": {Label: "Class", Value: d.StorageClass},
		"region":        {Label: "Site", Value: dash(d.Site)},
		"project":       {Label: "Project", Value: d.Project},
		"status":        {Label: "Status", Value: status},
	}
}

// resolveDeleteTarget picks the single key token refers to among the API
// keys and the saved keys. Saved keys resolve a name to an access key ID;
// when the API does not list the key, the saved metadata is used as target
// (the API still needs the username).
func resolveDeleteTarget(token string, keys []apiKey, saved map[string]config.StoredAccessKey, filter listFilter) (apiKey, error) {
	matches := findAPIKeys(token, keys, filter)
	savedName, savedKey, hasSaved := findSavedKey(token, saved)
	if len(matches) == 0 && hasSaved && savedKey.AccessKeyID != "" {
		matches = findAPIKeys(savedKey.AccessKeyID, keys, filter)
	}
	switch len(matches) {
	case 1:
		return filter.withSite(matches[0]), nil
	case 0:
		if hasSaved && filter.matchSaved(savedKey) {
			k := apiKey{
				Name: savedName, Username: savedKey.Username, AccessKeyID: savedKey.AccessKeyID,
				StorageClass: savedKey.StorageClass, Site: savedKey.Site, Project: firstNonEmptyStr(filter.Project, savedKey.ProjectID),
				Access: savedKey.Scope,
			}
			if filter.Site != "" {
				k.Site = filter.Site
			}
			return k, nil
		}
		return apiKey{}, exitcode.Errorf(exitcode.NotFound, "access key %q not found; run 'lsh s3 access-keys list' (add --project to search one project)", token)
	}
	return apiKey{}, ambiguousKeys(token, matches)
}

func runAccessKeysDelete(cmd *cobra.Command, args []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()
	token := strings.TrimSpace(args[0])

	class, err := storageClassFlag(cmd)
	if err != nil {
		return printErr(err)
	}
	site, err := regionFlag(cmd)
	if err != nil {
		return printErr(err)
	}
	yes, _ := cmd.Flags().GetBool(flagYes)
	filter := listFilter{StorageClass: class, Site: site, Project: projectFlag(cmd)}
	_, _, savedKeys := savedKeysByID(cmd)
	_, savedKey, hasSaved := findSavedKey(token, savedKeys)

	r := newResolver(cmd)
	if r.EndpointURL != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "access keys are managed through the Latitude API; unset --endpoint-url"))
	}
	api := newKeysAPI()
	projects := []string{filter.Project}
	if filter.Project == "" {
		if hasSaved && savedKey.ProjectID != "" {
			projects = []string{savedKey.ProjectID}
		} else {
			projects, _, err = teamProjects(ctx, r)
			if err != nil {
				return printErr(err)
			}
		}
	}
	keys, err := api.listProjects(ctx, projects)
	if err != nil {
		return printErr(err)
	}
	target, err := resolveDeleteTarget(token, keys, savedKeys, filter)
	if err != nil {
		return printErr(err)
	}
	if target.StorageClass == "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "the storage class of %q is unknown; pass --storage-class standard|high_performance", token))
	}

	res := deleteKeyResult{
		Name: target.Name, AccessKeyID: target.AccessKeyID, Username: target.Username,
		StorageClass: target.StorageClass, Site: strings.ToUpper(target.Site), Project: target.Project, DryRun: dryRun(),
	}
	objectstorage.Hintf("access key %q  id=%s  class=%s  site=%s  project=%s", dash(res.Name), res.AccessKeyID, res.StorageClass, dash(res.Site), res.Project)

	if !dryRun() {
		if err := objectstorage.ConfirmOrRefuse(cmd, yes, fmt.Sprintf("Delete access key %q (%s)? Anything using it stops working", dash(res.Name), res.AccessKeyID)); err != nil {
			return printErr(err)
		}
		if err := api.delete(ctx, target.Username, target.StorageClass, target.Project, target.Site); err != nil {
			return printErr(err)
		}
		res.Deleted = true
		profileName, removed, err := objectstorage.ForgetKeyByID(profileFlag(cmd), target.AccessKeyID)
		if err != nil {
			lsh.LogDebugf("[s3] could not update the profile after deleting %s: %v", target.AccessKeyID, err)
		}
		res.Forgotten = removed
		if len(removed) > 0 {
			objectstorage.Hintf("removed saved key(s) %s from profile %s", strings.Join(quoteAll(removed), ", "), profileName)
		}
	}

	if isHuman() {
		prefix := ""
		if res.DryRun {
			prefix = "(dryrun) "
		}
		fmt.Printf("%sdelete_access_key: %s (%s)\n", prefix, dash(res.Name), res.AccessKeyID)
		return nil
	}
	render([]renderer.ResponseData{res})
	return nil
}

func quoteAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return out
}
