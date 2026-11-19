package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysGetCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "get <name|access-key-id>",
		Aliases: []string{"show", "describe"},
		Short:   "Show one access key",
		Long: `Show one access key by name, access key ID or username. The API is searched
first (every project unless --project is given), then the keys saved in the
active profile. The secret is never shown: it only exists in the create output.`,
		Example: `  lsh s3 access-keys get ci-deploy
  lsh s3 access-keys get XL68DDURVGUUOULWPCAE -o json`,
		Args: cobra.ExactArgs(1),
		RunE: runAccessKeysGet,
	})
	addProjectFlag(cmd, true, "search this project only (ID or slug)")
	cmd.Flags().StringP("storage-class", "c", "", "disambiguate by storage class: standard or high_performance")
	cmd.Flags().String("region", "", "disambiguate by site (e.g. DAL, TYO4)")
	return cmd
}

// findAPIKeys returns the API keys matching token by name, access key ID or
// username, narrowed by the filter.
func findAPIKeys(token string, keys []apiKey, filter listFilter) []apiKey {
	var out []apiKey
	for _, k := range keys {
		if !filter.matchAPI(k) {
			continue
		}
		if k.AccessKeyID == token || k.Username == token || strings.EqualFold(k.Name, token) {
			out = append(out, k)
		}
	}
	return out
}

// findSavedKey looks a saved key up by name or access key ID.
func findSavedKey(token string, keys map[string]config.StoredAccessKey) (string, config.StoredAccessKey, bool) {
	if k, ok := keys[token]; ok {
		return token, k, true
	}
	for name, k := range keys {
		if k.AccessKeyID == token {
			return name, k, true
		}
	}
	return "", config.StoredAccessKey{}, false
}

// ambiguousKeys formats the exit-2 error for several matching keys.
func ambiguousKeys(token string, keys []apiKey) error {
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("  %-22s class=%s site=%s project=%s", k.AccessKeyID, k.StorageClass, dash(strings.ToUpper(k.Site)), k.Project))
	}
	return exitcode.Errorf(exitcode.Usage, "%q matches %d access keys:\n%s\ndisambiguate with the access key ID or --storage-class / --region / --project", token, len(keys), strings.Join(lines, "\n"))
}

func runAccessKeysGet(cmd *cobra.Command, args []string) error {
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
	filter := listFilter{StorageClass: class, Site: site, Project: projectFlag(cmd)}
	savedByID, profileName, savedKeys := savedKeysByID(cmd)

	// Saved keys let a name or ID resolve to a project even without --project.
	savedName, savedKey, hasSaved := findSavedKey(token, savedKeys)

	r := newResolver(cmd)
	api := newKeysAPI()
	projects := []string{filter.Project}
	slugs := map[string]string{}
	if filter.Project == "" {
		if hasSaved && savedKey.ProjectID != "" {
			projects = []string{savedKey.ProjectID}
		} else {
			projects, slugs, err = teamProjects(ctx, r)
			if err != nil {
				return printErr(err)
			}
		}
	}
	keys, listErr := api.listProjects(ctx, projects)
	if listErr != nil && !hasSaved {
		return printErr(listErr)
	}
	if listErr != nil {
		lsh.LogDebugf("[s3] API listing failed, falling back to the saved key: %v", listErr)
	}

	matches := findAPIKeys(token, keys, filter)
	if len(matches) == 0 && hasSaved && savedKey.AccessKeyID != "" {
		matches = findAPIKeys(savedKey.AccessKeyID, keys, filter)
	}
	switch {
	case len(matches) > 1:
		return printErr(ambiguousKeys(token, matches))
	case len(matches) == 1:
		row := newKeyRow(filter.withSite(matches[0]), savedByID, slugs)
		if isHuman() {
			printKeyDetails(os.Stdout, row)
			return nil
		}
		render([]renderer.ResponseData{row})
		return nil
	}

	if !hasSaved {
		return printErr(exitcode.Errorf(exitcode.NotFound, "access key %q not found in the API nor in profile %s; run 'lsh s3 access-keys list'", token, dash(profileName)))
	}
	row := newSavedKeyRow(savedName, savedKey, profileName)
	if listErr == nil {
		row.API = "missing on API"
		objectstorage.Warnf("access key %q (%s) is saved in profile %s but the API does not list it; if it was deleted run 'lsh s3 access-keys forget %s'", savedName, savedKey.AccessKeyID, profileName, savedName)
	}
	if isHuman() {
		printSavedKeyDetails(os.Stdout, row)
		return nil
	}
	render([]renderer.ResponseData{row})
	return nil
}

// printKeyDetails prints the human view of one API key as Label: value lines.
func printKeyDetails(w io.Writer, r keyRow) {
	fmt.Fprintf(w, "Name:            %s\n", r.Name)
	fmt.Fprintf(w, "Access Key ID:   %s\n", r.AccessKeyID)
	fmt.Fprintf(w, "Scope:           %s\n", r.scopeCell())
	buckets := r.bucketsCell()
	if r.Access != config.ScopeFullAccess && len(r.Buckets) > 0 {
		buckets = strings.Join(r.Buckets, ", ")
	}
	fmt.Fprintf(w, "Buckets:         %s\n", buckets)
	fmt.Fprintf(w, "Class:           %s\n", r.StorageClass)
	fmt.Fprintf(w, "Site:            %s\n", dash(r.Site))
	fmt.Fprintf(w, "Project:         %s\n", dash(r.Project))
	fmt.Fprintf(w, "Status:          %s\n", dash(r.Status))
	if r.Username != "" {
		fmt.Fprintf(w, "Username:        %s\n", r.Username)
	}
	if r.CreatedAt != "" {
		fmt.Fprintf(w, "Created:         %s\n", r.CreatedAt)
	}
	saved := "no"
	if r.Saved {
		saved = fmt.Sprintf("yes (as %q)", r.SavedAs)
	}
	fmt.Fprintf(w, "Saved:           %s\n", saved)
}

// printSavedKeyDetails prints the human view of one saved key (no secret).
func printSavedKeyDetails(w io.Writer, r savedKeyRow) {
	fmt.Fprintf(w, "Name:            %s\n", r.Name)
	fmt.Fprintf(w, "Access Key ID:   %s\n", r.AccessKeyID)
	fmt.Fprintf(w, "Scope:           %s\n", dash(r.Scope))
	buckets := r.bucketsCell()
	if r.Scope != config.ScopeFullAccess && len(r.Buckets) > 0 {
		buckets = formatPerms(r.Buckets)
	}
	fmt.Fprintf(w, "Buckets:         %s\n", buckets)
	fmt.Fprintf(w, "Class:           %s\n", dash(r.StorageClass))
	fmt.Fprintf(w, "Site:            %s\n", dash(r.Site))
	fmt.Fprintf(w, "Project:         %s\n", dash(r.ProjectID))
	if r.Username != "" {
		fmt.Fprintf(w, "Username:        %s\n", r.Username)
	}
	fmt.Fprintf(w, "Source:          %s\n", dash(r.Source))
	fmt.Fprintf(w, "Created:         %s\n", dash(r.CreatedAt))
	fmt.Fprintf(w, "Profile:         %s\n", dash(r.Profile))
	if r.API != "" {
		fmt.Fprintf(w, "API:             %s\n", r.API)
	}
}
