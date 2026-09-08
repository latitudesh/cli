package s3

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysRotateCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:   "rotate <saved-name>",
		Short: "Create a new key with the same scope as a saved key and save it in its place",
		Long: `Rotate a saved access key: a new key with the same storage class, site,
project and scope (fullaccess or the same buckets and permissions) is created
and saved under the same name, replacing the old one on this machine.

The old key keeps working until it is deleted: pass --delete-old to delete it
right away (after confirmation), or delete it later once every consumer has
the new secret.`,
		Example: `  lsh s3 access-keys rotate ci-deploy
  lsh s3 access-keys rotate ci-deploy --delete-old --yes
  lsh s3 access-keys rotate ci-deploy --name ci-deploy-2026q4`,
		Args: cobra.ExactArgs(1),
		RunE: runAccessKeysRotate,
	})
	addProjectFlag(cmd, true, "project of the key (ID or slug) when the saved key does not record it")
	cmd.Flags().Bool("delete-old", false, "delete the old key on the API after the new one is saved")
	cmd.Flags().String("name", "", "name of the new key (default: the saved name with a -<yyyymmdd> suffix)")
	cmd.Flags().String("region", "", "site of the key (e.g. TYO4) when the saved high_performance key does not record it")
	addYesFlag(cmd)
	return cmd
}

// rotatedName appends the date suffix used for the new key by default, plus a
// sequence number from the second rotation of the same day on (the API rejects
// a duplicate name). The result always fits the API's name limit, so a long
// alias does not produce a request the API refuses.
func rotatedName(base string, now time.Time, seq int) string {
	base = sanitizeKeyPart(base)
	if base == "" {
		base = "lsh-key"
	}
	suffix := "-" + now.UTC().Format("20060102")
	if seq > 1 {
		suffix += "-" + strconv.Itoa(seq)
	}
	if len(base)+len(suffix) > maxAccessKeyNameLen {
		base = strings.Trim(base[:maxAccessKeyNameLen-len(suffix)], "-")
	}
	return base + suffix
}

// rotateCreateKey creates the new key, working around a name that is already
// taken. Unlike create, rotate does not fall back to a random pet name: the
// name is meant to stay recognisable, so it grows a sequence number instead.
func rotateCreateKey(ctx context.Context, cmd *cobra.Command, req accessKeyRequest, base string, autoName bool) (*createdAccessKey, error) {
	for seq := 2; ; seq++ {
		created, err := createAccessKeyRetrying(ctx, cmd, req, false)
		if err == nil || !autoName || !isNameConflict(err) || seq > 9 {
			return created, err
		}
		req.Name = rotatedName(base, time.Now(), seq)
	}
}

// rotateResult is the structured output of rotate.
type rotateResult struct {
	Name           string `json:"name"`
	SavedAs        string `json:"saved_as"`
	Profile        string `json:"profile"`
	OldAccessKeyID string `json:"old_access_key_id"`
	NewAccessKeyID string `json:"new_access_key_id,omitempty"`
	StorageClass   string `json:"storage_class"`
	Site           string `json:"site,omitempty"`
	Project        string `json:"project"`
	Scope          string `json:"scope"`
	OldDeleted     bool   `json:"old_deleted"`
	DryRun         bool   `json:"dry_run,omitempty"`
}

func (r rotateResult) TableRow() table.Row {
	status := "rotated"
	if r.DryRun {
		status = "dryrun"
	}
	return table.Row{
		"name":              {Label: "Name", Value: r.Name},
		"old_access_key_id": {Label: "Old Access Key ID", Value: r.OldAccessKeyID},
		"new_access_key_id": {Label: "New Access Key ID", Value: dash(r.NewAccessKeyID)},
		"storage_class":     {Label: "Class", Value: r.StorageClass},
		"region":            {Label: "Site", Value: dash(r.Site)},
		"status":            {Label: "Status", Value: status},
	}
}

// rotateRequest builds the create request that mirrors a saved key.
func rotateRequest(k config.StoredAccessKey, project, site, newName string) (accessKeyRequest, error) {
	if k.Scope == config.ScopeUnknown || k.Scope == "" {
		return accessKeyRequest{}, exitcode.Errorf(exitcode.Usage, "the saved key has unknown scope, so its scope cannot be reproduced; create a new key with 'lsh s3 access-keys create' instead")
	}
	req := accessKeyRequest{
		Project:      firstNonEmptyStr(k.ProjectID, project),
		StorageClass: k.StorageClass,
		Site:         strings.ToUpper(firstNonEmptyStr(site, k.Site)),
		Name:         newName,
		Scope:        k.Scope,
	}
	if k.Scope == config.ScopeLimitedAccess {
		req.Buckets = make(map[string]string, len(k.Buckets))
		for id, perm := range k.Buckets {
			req.Buckets[id] = perm
		}
	}
	if req.Project == "" {
		return accessKeyRequest{}, exitcode.Errorf(exitcode.Usage, "the saved key does not record its project; pass --project <id|slug>")
	}
	if req.StorageClass == "" {
		return accessKeyRequest{}, exitcode.Errorf(exitcode.Usage, "the saved key does not record its storage class; re-import it with --storage-class or create a new key")
	}
	return req, nil
}

func runAccessKeysRotate(cmd *cobra.Command, args []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()
	savedName := strings.TrimSpace(args[0])

	site, err := regionFlag(cmd)
	if err != nil {
		return printErr(err)
	}
	deleteOld, _ := cmd.Flags().GetBool("delete-old")
	yes, _ := cmd.Flags().GetBool(flagYes)
	newName, _ := cmd.Flags().GetString("name")
	newName = strings.TrimSpace(newName)

	_, profileName, p, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return printErr(err)
	}
	old, ok := p.ObjectStorageKeys()[savedName]
	if !ok {
		return printErr(exitcode.Errorf(exitcode.NotFound, "no saved access key named %q in profile %s; run 'lsh s3 access-keys list --saved'", savedName, profileName))
	}
	autoName := newName == ""
	if autoName {
		newName = rotatedName(savedName, time.Now(), 1)
	} else if err := validateAccessKeyName(newName); err != nil {
		return printErr(err)
	}
	req, err := rotateRequest(old, projectFlag(cmd), site, newName)
	if err != nil {
		return printErr(err)
	}
	if err := req.validate(); err != nil {
		return printErr(err)
	}
	// Do not create a key we would then refuse to finish handling.
	if deleteOld && !yes && !dryRun() && !objectstorage.CanPrompt(cmd) {
		return printErr(exitcode.Errorf(exitcode.Refused, "--delete-old asks for confirmation — pass --yes in a non-interactive session"))
	}

	res := rotateResult{
		Name: newName, SavedAs: savedName, Profile: profileName, OldAccessKeyID: old.AccessKeyID,
		StorageClass: req.StorageClass, Site: req.Site, Project: req.Project, Scope: req.Scope, DryRun: dryRun(),
	}
	deleteHint := fmt.Sprintf("lsh s3 access-keys delete %s --project %s --storage-class %s", old.AccessKeyID, req.Project, req.StorageClass)
	if req.StorageClass == objectstorage.ClassHighPerformance {
		deleteHint += " --region " + req.Site
	}

	if dryRun() {
		if isHuman() {
			fmt.Printf("(dryrun) rotate: %s -> new key %q (class=%s site=%s project=%s scope=%s), saved as %q in profile %s\n",
				old.AccessKeyID, newName, req.StorageClass, dash(req.Site), req.Project, req.Scope, savedName, profileName)
			if deleteOld {
				fmt.Printf("(dryrun) delete_access_key: %s\n", old.AccessKeyID)
			}
			return nil
		}
		render([]renderer.ResponseData{res})
		return nil
	}

	api := newKeysAPI()
	created, err := rotateCreateKey(ctx, cmd, req, savedName, autoName)
	if err != nil {
		return printErr(err)
	}
	res.Name = created.Name
	res.NewAccessKeyID = created.AccessKeyID
	// Record the project as a proj_ ID: the old key's when it has one,
	// otherwise --project resolved through the project's buckets.
	projectID := old.ProjectID
	if projectID == "" {
		projectID = resolveProjectID(ctx, newResolver(cmd), req.Project)
	}
	stored := created.stored(projectID, req.Buckets, config.KeySourceCreate)
	// Rotate replaces the saved key in place (same name), unlike create.
	if _, err := objectstorage.SaveKey(profileFlag(cmd), savedName, stored); err != nil {
		// The old key is still live and still saved, so the new one can simply
		// be removed: rotating again is cheaper than handling a printed secret.
		// The view renders the scope from the plan, so the request's buckets
		// have to be there for the fallback print to be complete.
		created.Buckets = req.Buckets
		reportUnsavedKey(ctx, os.Stderr, req, created, &createPlan{Request: req, Buckets: rotateScopedBuckets(req)}, savedName, false, err)
		objectstorage.Hintf("the old key %s is untouched and still saved as %q", old.AccessKeyID, savedName)
		return printErr(objectstorage.Humanize(err, nil, nil))
	}
	objectstorage.Hintf("new key %s saved as %q in profile %s (replacing %s)", created.AccessKeyID, savedName, profileName, old.AccessKeyID)

	if deleteOld {
		username := old.Username
		if username == "" {
			if keys, err := api.list(ctx, req.Project); err == nil {
				for _, k := range keys {
					if k.AccessKeyID == old.AccessKeyID {
						username = k.Username
						break
					}
				}
			}
		}
		switch {
		case username == "":
			objectstorage.Warnf("could not find the username of the old key %s on the API; delete it later with: %s", old.AccessKeyID, deleteHint)
		default:
			err := objectstorage.ConfirmOrRefuse(cmd, yes, fmt.Sprintf("Delete the old access key %s now? Anything still using it stops working", old.AccessKeyID))
			if err != nil {
				objectstorage.Warnf("old key %s kept; delete it later with: %s", old.AccessKeyID, deleteHint)
			} else if err := api.delete(ctx, username, req.StorageClass, req.Project, req.Site); err != nil {
				objectstorage.Warnf("could not delete the old key %s: %v — delete it later with: %s", old.AccessKeyID, err, deleteHint)
			} else {
				res.OldDeleted = true
			}
		}
	} else {
		objectstorage.Hintf("old key %s kept so existing consumers keep working; once they have the new secret run: %s", old.AccessKeyID, deleteHint)
	}

	if isHuman() {
		fmt.Printf("rotate: %s -> %s (saved as %q in profile %s)\n", old.AccessKeyID, created.AccessKeyID, savedName, profileName)
		if res.OldDeleted {
			fmt.Printf("delete_access_key: %s\n", old.AccessKeyID)
		}
		return nil
	}
	render([]renderer.ResponseData{res})
	return nil
}

// rotateScopedBuckets rebuilds the plan's bucket list from the request. Rotate
// copies the old key's scope, which is stored as bkt_ IDs, so only the IDs and
// permissions are known — enough for the printed scope of an unsaved key.
func rotateScopedBuckets(req accessKeyRequest) []scopedBucket {
	if len(req.Buckets) == 0 {
		return nil
	}
	ids := make([]string, 0, len(req.Buckets))
	for id := range req.Buckets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]scopedBucket, 0, len(ids))
	for _, id := range ids {
		out = append(out, scopedBucket{Bucket: &objectstorage.Bucket{ID: id, Name: id}, Permission: req.Buckets[id]})
	}
	return out
}
