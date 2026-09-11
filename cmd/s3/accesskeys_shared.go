package s3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// accessKeyRequest describes an access key to create through the API.
type accessKeyRequest struct {
	Project      string            // project ID or slug (required)
	StorageClass string            // standard | high_performance (required)
	Site         string            // site slug; required for high_performance
	Name         string            // key name (normalized server-side)
	Scope        string            // config.ScopeFullAccess | config.ScopeLimitedAccess
	Buckets      map[string]string // bkt_ id → rw|readonly (limited_access only)
}

// createdAccessKey is the normalized result of a create call, independent of
// the backend's field names (Wasabi: access_key_id/secret_access_key; VAST:
// access_key/secret_key).
type createdAccessKey struct {
	Name            string `json:"name"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Username        string `json:"username,omitempty"`
	Status          string `json:"status,omitempty"`
	StorageClass    string `json:"storage_class"`
	Site            string `json:"site,omitempty"`
	Project         string `json:"project"`
	Scope           string `json:"scope"`
	// Buckets lists bucket display names → permission for limited keys. Right
	// after createAccessKey it holds the bkt_ IDs of the request; the create
	// command swaps them for display names before printing.
	Buckets map[string]string `json:"buckets,omitempty"`
	// Endpoint and SigningRegion are filled by the caller from the buckets
	// the key covers (the API does not return them).
	Endpoint      string `json:"endpoint,omitempty"`
	SigningRegion string `json:"signing_region,omitempty"`
}

// stored converts the created key into the config.json representation.
func (k createdAccessKey) stored(projectID string, bucketPerms map[string]string, source string) config.StoredAccessKey {
	return config.StoredAccessKey{
		AccessKeyID:     k.AccessKeyID,
		SecretAccessKey: k.SecretAccessKey,
		StorageClass:    k.StorageClass,
		Site:            k.Site,
		ProjectID:       projectID,
		Scope:           k.Scope,
		Buckets:         bucketPerms,
		Username:        k.Username,
		Source:          source,
		CreatedAt:       time.Now().UTC(),
	}
}

// createAccessKey calls POST /storage/access_keys with the CLI's API client.
// mb and configure call it to offer a key right after bucket creation.
func createAccessKey(ctx context.Context, cmd *cobra.Command, req accessKeyRequest) (*createdAccessKey, error) {
	_ = cmd
	return newKeysAPI().create(ctx, req)
}

// provisioningRetryDelays are the waits before re-sending a create that failed
// because the backend had not registered the new key yet. Creating a key is a
// multi-step operation on the provider side (create the user, create the key,
// then attach the bucket policy), and the steps do not become visible to each
// other instantly, so the first attempt can fail with a 404 that succeeds a
// few seconds later. The API rolls its own steps back before returning the
// error, so re-sending the same request is safe.
var provisioningRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

// createAccessKeyRetrying creates the key, retrying two failures the API
// itself considers recoverable: a duplicate name (only when the name was
// auto-generated — a fresh pet name is rolled, the way the dashboard avoids
// collisions) and the propagation lag above (same request, after a wait).
func createAccessKeyRetrying(ctx context.Context, cmd *cobra.Command, req accessKeyRequest, autoName bool) (*createdAccessKey, error) {
	return retryCreate(ctx, req, autoName, func(r accessKeyRequest) (*createdAccessKey, error) {
		return createAccessKey(ctx, cmd, r)
	})
}

// retryCreate holds the retry policy of createAccessKeyRetrying, with the
// create call injected so the policy can be tested without the API.
func retryCreate(ctx context.Context, req accessKeyRequest, autoName bool, create func(accessKeyRequest) (*createdAccessKey, error)) (*createdAccessKey, error) {
	namesLeft, waited := 5, 0
	for {
		created, err := create(req)
		if err == nil {
			return created, nil
		}
		switch {
		case autoName && isNameConflict(err) && namesLeft > 1:
			namesLeft--
			req.Name = generateKeyName(req.StorageClass)
		case isProvisioningLag(err) && waited < len(provisioningRetryDelays):
			delay := provisioningRetryDelays[waited]
			waited++
			if waited == 1 {
				objectstorage.Hintf("the storage backend has not registered the key yet; retrying in %s…", delay)
			}
			if waitErr := sleepCtx(ctx, delay); waitErr != nil {
				return nil, waitErr
			}
		case isProvisioningLag(err):
			return nil, exitcode.Errorf(exitcode.Of(err), "%v — the storage backend did not register the key in time; retry in a few seconds", err)
		default:
			return nil, err
		}
	}
}

// isNameConflict reports whether err is the API rejecting a duplicate key name.
func isNameConflict(err error) bool {
	m := strings.ToLower(err.Error())
	if !strings.Contains(m, "name") {
		return false
	}
	return strings.Contains(m, "taken") || strings.Contains(m, "already") ||
		strings.Contains(m, "exist") || strings.Contains(m, "unique") ||
		strings.Contains(m, "in use")
}

// isProvisioningLag reports whether err is the API surfacing a backend that is
// still converging: STORAGE_RESOURCE_NOT_FOUND ("Storage resource not found.")
// while the new key propagates, or STORAGE_UNAVAILABLE. Both are transient and
// the same request succeeds once the backend catches up.
func isProvisioningLag(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "storage resource not found") ||
		strings.Contains(m, "temporarily unavailable")
}

// sleepCtx waits for d, returning early (with the right exit code) if the
// context is cancelled by Ctrl-C or a deadline.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return exitcode.Errorf(exitcode.Interrupted, "interrupted")
		}
		return exitcode.Errorf(exitcode.Generic, "timed out waiting for the storage backend: %v", ctx.Err())
	}
}

// keysAPI wraps the Latitude API calls of the access-keys commands so the
// command cores can run against a test server. Create and delete go through
// the SDK; listing reads the raw JSON:API document because the generated
// model drops `buckets[]` and `access` (see list).
type keysAPI struct {
	sdk  *sdk.Latitudesh
	opts []operations.Option
	// baseURL, token and apiVersion configure the raw GET (same settings the
	// SDK client is built with).
	baseURL    string
	token      string
	apiVersion string
	http       *http.Client
}

// newKeysAPI builds the wrapper around the CLI's authenticated client.
func newKeysAPI() *keysAPI {
	host := viper.GetString("hostname")
	if host == "" {
		host = "api.latitude.sh"
	}
	scheme := viper.GetString("scheme")
	if scheme == "" {
		scheme = "https"
	}
	return &keysAPI{
		sdk:        apiClient(),
		opts:       []operations.Option{operations.WithRetries(lsh.RetryConfig())},
		baseURL:    scheme + "://" + host,
		token:      viper.GetString("Authorization"),
		apiVersion: viper.GetString("api-version"),
	}
}

// rawGet performs an authenticated GET and returns the body. Non-2xx
// responses are mapped through objectstorage.HumanizeAPI (as an SDK
// *components.APIError) so exit codes match the SDK calls.
func (a *keysAPI) rawGet(ctx context.Context, path string, query url.Values, what string) ([]byte, error) {
	if a.token == "" {
		return nil, exitcode.Errorf(exitcode.Credentials, "not logged in — run 'lsh login' first")
	}
	u := a.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(a.token, "Bearer "))
	req.Header.Set("Accept", "application/vnd.api+json, application/json")
	req.Header.Set("User-Agent", "lsh/"+version.Version)
	if a.apiVersion != "" {
		req.Header.Set("API-Version", a.apiVersion)
	}
	client := a.http
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, what)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, objectstorage.HumanizeAPI(&components.APIError{Message: "API error", StatusCode: resp.StatusCode, Body: string(body), RawResponse: resp}, what)
	}
	return body, nil
}

// validate checks the request before it reaches the API.
func (r accessKeyRequest) validate() error {
	switch {
	case r.Project == "":
		return exitcode.Errorf(exitcode.Usage, "a project is required to create an access key; pass --project")
	case r.StorageClass != objectstorage.ClassStandard && r.StorageClass != objectstorage.ClassHighPerformance:
		return exitcode.Errorf(exitcode.Usage, "invalid storage class %q (use standard or high_performance)", r.StorageClass)
	case r.Name == "":
		return exitcode.Errorf(exitcode.Usage, "an access key name is required; pass --name")
	case r.StorageClass == objectstorage.ClassHighPerformance && r.Site == "":
		return exitcode.Errorf(exitcode.Usage, "--region <site> is required for high_performance keys (a Latitude site slug such as TYO4 or DAL)")
	case r.Scope == config.ScopeLimitedAccess && len(r.Buckets) == 0:
		return exitcode.Errorf(exitcode.Usage, "a limited_access key needs at least one --bucket")
	case r.Scope != config.ScopeLimitedAccess && r.Scope != config.ScopeFullAccess:
		return exitcode.Errorf(exitcode.Usage, "invalid access scope %q", r.Scope)
	}
	for id, perm := range r.Buckets {
		if perm != config.PermissionRW && perm != config.PermissionReadOnly {
			return exitcode.Errorf(exitcode.Usage, "invalid permission %q for bucket %s (use rw or readonly)", perm, id)
		}
	}
	return nil
}

// buildCreateRequest converts the request into the SDK body. Bucket
// permissions are sorted so the payload (and dry-run output) is stable.
func buildCreateRequest(req accessKeyRequest) operations.PostStorageAccessKeysRequestBody {
	attrs := operations.PostStorageAccessKeysAttributes{
		Project:               req.Project,
		AccessKeyStorageClass: operations.AccessKeyStorageClass(req.StorageClass),
		Name:                  req.Name,
		AccessScope:           operations.AccessScope(req.Scope),
		Region:                req.Site,
	}
	if req.Scope == config.ScopeLimitedAccess {
		ids := make([]string, 0, len(req.Buckets))
		for id := range req.Buckets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			attrs.BucketPermissions = append(attrs.BucketPermissions, operations.BucketPermissions{
				BucketID:   id,
				Permission: operations.Permission(req.Buckets[id]),
			})
		}
	}
	return operations.PostStorageAccessKeysRequestBody{
		Data: operations.PostStorageAccessKeysData{
			Type:       operations.PostStorageAccessKeysTypeAccessKeys,
			Attributes: attrs,
		},
	}
}

// create performs the POST and normalizes the response.
func (a *keysAPI) create(ctx context.Context, req accessKeyRequest) (*createdAccessKey, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	resp, err := a.sdk.ObjectStorage.PostStorageAccessKeys(ctx, buildCreateRequest(req), a.opts...)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, "")
	}
	return normalizeCreated(resp, req)
}

// normalizeCreated maps both backend shapes onto createdAccessKey.
func normalizeCreated(resp *operations.PostStorageAccessKeysResponse, req accessKeyRequest) (*createdAccessKey, error) {
	var ak *operations.AccessKey
	if resp != nil && resp.Object != nil && resp.Object.Data != nil && resp.Object.Data.Attributes != nil {
		ak = resp.Object.Data.Attributes.AccessKey
	}
	if ak == nil {
		return nil, exitcode.Errorf(exitcode.Generic, "the API created the access key but returned no credentials; check 'lsh s3 access-keys list --project %s'", req.Project)
	}
	k := &createdAccessKey{
		Name:            firstNonEmptyStr(ptrStr(ak.Name), req.Name),
		AccessKeyID:     firstNonEmptyStr(ptrStr(ak.AccessKeyID), ptrStr(ak.AccessKey)),
		SecretAccessKey: firstNonEmptyStr(ptrStr(ak.SecretAccessKey), ptrStr(ak.SecretKey)),
		Username:        ptrStr(ak.Username),
		Status:          ptrStr(ak.Status),
		StorageClass:    req.StorageClass,
		Site:            req.Site,
		Project:         req.Project,
		Scope:           req.Scope,
	}
	if k.Scope == config.ScopeLimitedAccess && len(req.Buckets) > 0 {
		k.Buckets = make(map[string]string, len(req.Buckets))
		for id, perm := range req.Buckets {
			k.Buckets[id] = perm
		}
	}
	if k.AccessKeyID == "" || k.SecretAccessKey == "" {
		return nil, exitcode.Errorf(exitcode.Generic, "the API response is missing the access key id or secret (backend %s); the key may exist: check 'lsh s3 access-keys list --project %s'", req.StorageClass, req.Project)
	}
	return k, nil
}

// apiKey is one access key as listed by the API, merged with the raw
// scope information (buckets[] and access) the SDK model drops.
type apiKey struct {
	Name         string
	Username     string
	AccessKeyID  string
	Status       string
	CreatedAt    string
	StorageClass string
	Site         string
	// Project is the project reference (ID or slug) the key was listed under.
	Project string
	// Access is fullaccess, rw, readonly or "" when unknown.
	Access string
	// Buckets are backend bucket names.
	Buckets []string
}

// scope maps the API access value onto the config scope constants.
func (k apiKey) scope() string {
	switch k.Access {
	case config.ScopeFullAccess:
		return config.ScopeFullAccess
	case config.PermissionRW, config.PermissionReadOnly:
		return config.ScopeLimitedAccess
	}
	return config.ScopeUnknown
}

// rawAccessKey is one record of GET /storage/access_keys as the API returns
// it (both storage classes share the shape; `buckets` and `access` are the
// fields the SDK model drops).
type rawAccessKey struct {
	Name        string   `json:"name"`
	Username    string   `json:"username"`
	AccessKeyID string   `json:"access_key_id"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"created_at"`
	Region      string   `json:"region"`
	Access      string   `json:"access"`
	Buckets     []string `json:"buckets"`
}

// list returns the project's keys of both storage classes with one GET
// /storage/access_keys call: the raw document carries every field the rows
// need, so the SDK call (which would drop buckets[]/access) is not made.
func (a *keysAPI) list(ctx context.Context, project string) ([]apiKey, error) {
	if project == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "a project is required to list access keys; pass --project")
	}
	what := fmt.Sprintf("project %q", project)
	body, err := a.rawGet(ctx, "/storage/access_keys", url.Values{"project": {project}}, what)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data map[string][]rawAccessKey `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, exitcode.Errorf(exitcode.Generic, "could not parse the access keys of %s: %v", what, err)
	}
	var out []apiKey
	for class, keys := range env.Data {
		for _, k := range keys {
			out = append(out, apiKey{
				Name: k.Name, Username: k.Username, AccessKeyID: k.AccessKeyID, Status: k.Status,
				CreatedAt: k.CreatedAt, Site: k.Region, Access: k.Access, Buckets: k.Buckets,
				StorageClass: class, Project: project,
			})
		}
	}
	sortAPIKeys(out)
	return out, nil
}

// listForBucket returns the keys covering one bucket (GET /storage/buckets/{id}/access_keys).
func (a *keysAPI) listForBucket(ctx context.Context, b *objectstorage.Bucket) ([]apiKey, error) {
	resp, err := a.sdk.ObjectStorage.GetStorageBucketAccessKeys(ctx, b.ID, a.opts...)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, fmt.Sprintf("bucket %s", b.Display()))
	}
	var out []apiKey
	if resp.Object != nil {
		for _, d := range resp.Object.Data {
			out = append(out, apiKey{
				Name: ptrStr(d.Name), Username: ptrStr(d.Username), AccessKeyID: ptrStr(d.AccessKeyID),
				Status: ptrStr(d.Status), CreatedAt: ptrStr(d.CreatedAt), Access: ptrStr(d.Access),
				StorageClass: b.StorageClass, Site: b.Site, Project: b.ProjectRef(),
				Buckets: []string{b.BucketName},
			})
		}
	}
	sortAPIKeys(out)
	return out, nil
}

// listProjects lists the keys of several projects, concatenated.
func (a *keysAPI) listProjects(ctx context.Context, projects []string) ([]apiKey, error) {
	var out []apiKey
	for _, p := range projects {
		keys, err := a.list(ctx, p)
		if err != nil {
			return nil, err
		}
		out = append(out, keys...)
	}
	return out, nil
}

// delete removes a key. site is only sent for high_performance keys (the API
// ignores it for standard ones).
func (a *keysAPI) delete(ctx context.Context, username, storageClass, project, site string) error {
	if username == "" {
		return exitcode.Errorf(exitcode.Generic, "the access key has no username; the API needs it to delete the key — run 'lsh s3 access-keys list --project %s' to refresh it", project)
	}
	if project == "" {
		return exitcode.Errorf(exitcode.Usage, "a project is required to delete an access key; pass --project")
	}
	var region *string
	if storageClass == objectstorage.ClassHighPerformance {
		if site == "" {
			return exitcode.Errorf(exitcode.Usage, "--region <site> is required to delete a high_performance key (a Latitude site slug such as TYO4)")
		}
		region = &site
	}
	_, err := a.sdk.ObjectStorage.DeleteStorageAccessKeysUsername(ctx, username, operations.PathParamStorageClass(storageClass), project, region, a.opts...)
	if err != nil {
		return objectstorage.HumanizeAPI(err, fmt.Sprintf("access key %q", username))
	}
	return nil
}

// resolveProjectID returns the proj_ ID for a project reference (ID or
// slug) by looking at the project's buckets; it falls back to the reference
// itself when nothing resolves so callers can still store something useful.
func resolveProjectID(ctx context.Context, r *objectstorage.Resolver, project string) string {
	if project == "" || strings.HasPrefix(project, "proj_") || r == nil || r.API == nil {
		return project
	}
	scoped := *r
	scoped.Project = project
	list, err := scoped.ListBuckets(ctx)
	if err != nil {
		lsh.LogDebugf("[s3] could not resolve project %q to an ID: %v", project, err)
		return project
	}
	for _, d := range list {
		if id := objectstorage.BucketFromData(d).ProjectID; id != "" {
			return id
		}
	}
	return project
}

// teamProjects returns the distinct projects (IDs, with a slug lookup for
// display) that own buckets in the team, or the one given as filter.
func teamProjects(ctx context.Context, r *objectstorage.Resolver) ([]string, map[string]string, error) {
	list, err := r.ListBuckets(ctx)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	slugs := map[string]string{}
	var out []string
	for _, d := range list {
		b := objectstorage.BucketFromData(d)
		ref := b.ProjectID
		if ref == "" {
			ref = b.ProjectSlug
		}
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
		if b.ProjectSlug != "" {
			slugs[ref] = b.ProjectSlug
		}
	}
	sort.Strings(out)
	return out, slugs, nil
}

func sortAPIKeys(keys []apiKey) {
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].Project != keys[j].Project {
			return keys[i].Project < keys[j].Project
		}
		if keys[i].Name != keys[j].Name {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].AccessKeyID < keys[j].AccessKeyID
	})
}

// bucketSpec is one parsed --bucket value: <name|bkt_id>[=rw|readonly].
type bucketSpec struct {
	Token      string
	Permission string
}

// bucketSpecUsage documents the --bucket grammar once for every command
// that accepts it (create, import).
const bucketSpecUsage = "bucket the key covers, as <name|bkt_id>[=rw|readonly] (':' also separates; 'write' = rw, 'ro'/'read' = readonly; default rw; repeatable or comma-separated; a bucket, never s3://bucket/key)"

// parseBucketSpec accepts "name", "name=rw", "name:readonly" (and "s3://"
// prefixes). The permission defaults to rw. Anything with a key part
// ("bucket/dir/file") is refused: keys grant access to whole buckets.
func parseBucketSpec(s string) (bucketSpec, error) {
	raw := strings.TrimSpace(s)
	raw = strings.TrimPrefix(raw, "s3://")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return bucketSpec{}, exitcode.Errorf(exitcode.Usage, "empty --bucket value")
	}
	if strings.Contains(raw, "/") {
		return bucketSpec{}, exitcode.Errorf(exitcode.Usage, "invalid --bucket %q: access keys cover whole buckets, not keys; pass the bucket only (<name|bkt_id>[=rw|readonly])", s)
	}
	token, perm := raw, config.PermissionRW
	if i := strings.LastIndexAny(raw, "=:"); i >= 0 {
		token, perm = raw[:i], strings.ToLower(strings.TrimSpace(raw[i+1:]))
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return bucketSpec{}, exitcode.Errorf(exitcode.Usage, "invalid --bucket %q: missing bucket name", s)
	}
	switch perm {
	case config.PermissionRW, "readwrite", "read-write", "write":
		perm = config.PermissionRW
	case config.PermissionReadOnly, "ro", "read-only", "read":
		perm = config.PermissionReadOnly
	default:
		return bucketSpec{}, exitcode.Errorf(exitcode.Usage, "invalid permission %q in --bucket %q (use <bucket>=rw or <bucket>=readonly)", perm, s)
	}
	return bucketSpec{Token: token, Permission: perm}, nil
}

// parseBucketSpecs parses every --bucket flag, rejecting duplicates.
func parseBucketSpecs(values []string) ([]bucketSpec, error) {
	out := make([]bucketSpec, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		// Allow comma-separated lists in one flag too.
		for _, part := range strings.Split(v, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			spec, err := parseBucketSpec(part)
			if err != nil {
				return nil, err
			}
			if seen[spec.Token] {
				return nil, exitcode.Errorf(exitcode.Usage, "bucket %q given more than once in --bucket", spec.Token)
			}
			seen[spec.Token] = true
			out = append(out, spec)
		}
	}
	return out, nil
}

// scopedBucket is a resolved bucket plus the permission requested on it.
type scopedBucket struct {
	Bucket     *objectstorage.Bucket
	Permission string
}

// validateSameGroup makes sure every bucket shares storage class and
// project, and (for high_performance) site. It returns exit 2 listing the
// groups otherwise, since one key cannot span them.
func validateSameGroup(buckets []scopedBucket) error {
	if len(buckets) < 2 {
		return nil
	}
	groups := map[string][]string{}
	var order []string
	for _, sb := range buckets {
		b := sb.Bucket
		key := fmt.Sprintf("class=%s project=%s", b.StorageClass, firstNonEmptyStr(b.ProjectID, b.ProjectSlug))
		if b.StorageClass == objectstorage.ClassHighPerformance {
			key += " site=" + strings.ToUpper(b.Site)
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], b.Name)
	}
	if len(groups) < 2 {
		return nil
	}
	lines := make([]string, 0, len(order))
	for _, k := range order {
		lines = append(lines, fmt.Sprintf("  %s: %s", k, strings.Join(groups[k], ", ")))
	}
	return exitcode.Errorf(exitcode.Usage, "one access key cannot cover buckets of different storage classes, sites or projects:\n%s\ncreate one key per group", strings.Join(lines, "\n"))
}

var keyNameUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)
var keyNameDashes = regexp.MustCompile(`-{2,}`)

// sanitizeKeyPart lower-cases s and replaces anything outside [a-z0-9-]
// with '-', collapsing runs.
func sanitizeKeyPart(s string) string {
	s = keyNameUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = keyNameDashes.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// maxAccessKeyNameLen is the API limit on an access key name.
const maxAccessKeyNameLen = 25

// capKeyName truncates a name to the API limit and drops a trailing hyphen.
func capKeyName(s string) string {
	if len(s) > maxAccessKeyNameLen {
		s = s[:maxAccessKeyNameLen]
	}
	return strings.Trim(s, "-")
}

// validateAccessKeyName rejects an explicit --name that the API would refuse,
// before the request is sent, with an actionable usage error.
func validateAccessKeyName(name string) error {
	if len(name) > maxAccessKeyNameLen {
		return exitcode.Errorf(exitcode.Usage, "access key name %q is %d characters; the API allows at most %d", name, len(name), maxAccessKeyNameLen)
	}
	return nil
}

// discardUnsavedKey deletes a key that was created but could not be stored in
// the active profile. The API returns the secret exactly once, so a key whose
// secret nobody holds is unusable and would linger with live permissions:
// removing it is the recovery path that neither discloses the secret nor
// leaves a credential behind. Callers fall back to printing the secret (on
// stderr) only when this fails, or when the user asked for it explicitly.
// discardKey is the indirection tests replace; production always deletes
// through the API.
var discardKey = discardUnsavedKey

func discardUnsavedKey(ctx context.Context, req accessKeyRequest, created *createdAccessKey) error {
	if created == nil || created.Username == "" {
		return exitcode.Errorf(exitcode.Generic, "the API did not return the key's username, which is required to delete it")
	}
	return newKeysAPI().delete(ctx, created.Username, req.StorageClass, req.Project, req.Site)
}

// reportUnsavedKey is the shared tail of a failed save: it removes the key when
// it can, and otherwise prints the only copy of the secret (always on stderr,
// never on stdout) together with the command that stores it. showSecret skips
// the removal because the user asked to keep and print the key.
func reportUnsavedKey(ctx context.Context, w io.Writer, req accessKeyRequest, created *createdAccessKey, plan *createPlan, name string, showSecret bool, saveErr error) {
	if !showSecret {
		if delErr := discardKey(ctx, req, created); delErr == nil {
			fmt.Fprintf(w, "warning: access key %q was created but could not be saved to the profile: %v\n", name, saveErr)
			fmt.Fprintf(w, "It was deleted again, so nothing was left behind and no secret was printed. Retry once the profile is writable.\n")
			return
		} else {
			fmt.Fprintf(w, "warning: access key %q could not be saved (%v) nor deleted again (%v), so its secret is printed below — it cannot be retrieved another way.\n", name, saveErr, delErr)
		}
	} else {
		fmt.Fprintf(w, "warning: access key %q was created but could not be saved to the profile: %v\n", name, saveErr)
	}
	printCreatedHuman(w, newCreatedKeyView(created, plan, true), true)
	fmt.Fprintf(w, "Save it once the profile is writable with: %s\n", importHint(name, created, req, created.Buckets))
}

// importHint renders the `access-keys import` command that stores a key whose
// secret could not be saved automatically. buckets maps bucket display names
// to permissions and is only used for a limited_access key.
func importHint(name string, created *createdAccessKey, req accessKeyRequest, buckets map[string]string) string {
	hint := fmt.Sprintf("lsh s3 access-keys import --name %s --access-key-id %s --project %s --storage-class %s",
		name, created.AccessKeyID, req.Project, req.StorageClass)
	if req.StorageClass == objectstorage.ClassHighPerformance && req.Site != "" {
		hint += " --region " + req.Site
	}
	if req.Scope == config.ScopeLimitedAccess && len(buckets) > 0 {
		names := make([]string, 0, len(buckets))
		for b := range buckets {
			names = append(names, b)
		}
		sort.Strings(names)
		for _, b := range names {
			hint += fmt.Sprintf(" --bucket %s=%s", b, buckets[b])
		}
		return hint
	}
	return hint + " --all-buckets"
}

// secretDecision says whether the freshly created secret may be printed and
// which warning (if any) explains an omission.
type secretDecision struct {
	Show    bool
	Warning string
}

// decideSecretDisplay implements the display rules of the plan (J3):
//   - human output, not saving: the secret is printed once in clear;
//   - saving (--save): only with --show-secret, in every format;
//   - structured output: only when -o was passed explicitly or --show-secret
//     (never because of LSH_OUTPUT/config), otherwise omitted with a warning.
func decideSecretDisplay(human, save, showSecret, outputExplicit bool) secretDecision {
	switch {
	case showSecret:
		return secretDecision{Show: true}
	case save:
		return secretDecision{Warning: "secret omitted: it was saved in the profile; pass --show-secret to print it"}
	case human:
		return secretDecision{Show: true}
	case outputExplicit:
		return secretDecision{Show: true}
	default:
		return secretDecision{Warning: "secret omitted: pass -o json explicitly or --show-secret"}
	}
}

// keyRow renders one API access key (list/get).
type keyRow struct {
	Name         string   `json:"name"`
	AccessKeyID  string   `json:"access_key_id"`
	Scope        string   `json:"scope"`
	Access       string   `json:"access,omitempty"`
	Buckets      []string `json:"buckets,omitempty"`
	StorageClass string   `json:"storage_class"`
	Site         string   `json:"site,omitempty"`
	Project      string   `json:"project,omitempty"`
	Status       string   `json:"status,omitempty"`
	Username     string   `json:"username,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
	Saved        bool     `json:"saved"`
	SavedAs      string   `json:"saved_as,omitempty"`
}

// newKeyRow builds the row, marking it saved when a profile key has the
// same access key ID.
func newKeyRow(k apiKey, saved map[string]string, slugs map[string]string) keyRow {
	project := k.Project
	if s, ok := slugs[project]; ok && s != "" {
		project = s
	}
	row := keyRow{
		Name: k.Name, AccessKeyID: k.AccessKeyID, Scope: k.scope(), Access: k.Access, Buckets: k.Buckets,
		StorageClass: k.StorageClass, Site: strings.ToUpper(k.Site), Project: project, Status: k.Status,
		Username: k.Username, CreatedAt: k.CreatedAt,
	}
	if name, ok := saved[k.AccessKeyID]; ok {
		row.Saved, row.SavedAs = true, name
	}
	return row
}

func (r keyRow) TableRow() table.Row {
	return table.Row{
		"name":          {Label: "Name", Value: r.Name},
		"access_key_id": {Label: "Access Key ID", Value: r.AccessKeyID},
		"scope":         {Label: "Scope", Value: r.scopeCell()},
		"buckets":       {Label: "Buckets", Value: r.bucketsCell(), MaxLength: 40},
		"storage_class": {Label: "Class", Value: r.StorageClass},
		"region":        {Label: "Site", Value: dash(r.Site)},
		"project":       {Label: "Project", Value: dash(r.Project)},
		"status":        {Label: "Status", Value: dash(r.Status)},
		"saved":         {Label: "Saved", Value: yesNo(r.Saved)},
	}
}

// scopeCell is the SCOPE column: the raw access (fullaccess|rw|readonly) or '-'.
func (r keyRow) scopeCell() string { return dash(r.Access) }

// bucketsCell is the BUCKETS column: "all" for fullaccess, the backend names
// otherwise (truncated).
func (r keyRow) bucketsCell() string {
	if r.Access == config.ScopeFullAccess {
		return "all"
	}
	if len(r.Buckets) == 0 {
		return emptyCell
	}
	return truncateText(strings.Join(r.Buckets, ", "), 40)
}

// savedKeyRow renders one saved (profile) key; never the secret.
type savedKeyRow struct {
	Name         string            `json:"name"`
	AccessKeyID  string            `json:"access_key_id"`
	StorageClass string            `json:"storage_class"`
	Site         string            `json:"site,omitempty"`
	ProjectID    string            `json:"project_id,omitempty"`
	Scope        string            `json:"scope"`
	Buckets      map[string]string `json:"buckets,omitempty"`
	Username     string            `json:"username,omitempty"`
	Source       string            `json:"source,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	Profile      string            `json:"profile,omitempty"`
	// API is "ok", "missing on API" or "" when no listing was available.
	API string `json:"api_status,omitempty"`
}

func newSavedKeyRow(name string, k config.StoredAccessKey, profile string) savedKeyRow {
	created := ""
	if !k.CreatedAt.IsZero() {
		created = k.CreatedAt.UTC().Format(time.RFC3339)
	}
	return savedKeyRow{
		Name: name, AccessKeyID: k.AccessKeyID, StorageClass: k.StorageClass, Site: strings.ToUpper(k.Site),
		ProjectID: k.ProjectID, Scope: k.Scope, Buckets: k.Buckets, Username: k.Username, Source: k.Source,
		CreatedAt: created, Profile: profile,
	}
}

func (r savedKeyRow) TableRow() table.Row {
	return table.Row{
		"name":          {Label: "Name", Value: r.Name},
		"access_key_id": {Label: "Access Key ID", Value: r.AccessKeyID},
		"storage_class": {Label: "Class", Value: dash(r.StorageClass)},
		"region":        {Label: "Site", Value: dash(r.Site)},
		"project":       {Label: "Project", Value: dash(r.ProjectID)},
		"scope":         {Label: "Scope", Value: dash(r.Scope)},
		"buckets":       {Label: "Buckets", Value: r.bucketsCell(), MaxLength: 40},
		"source":        {Label: "Source", Value: dash(r.Source)},
		"created_at":    {Label: "Created", Value: dash(r.CreatedAt)},
		"status":        {Label: "API", Value: dash(r.API)},
	}
}

// bucketsCell renders "bkt_a=rw bkt_b=readonly" (sorted, truncated).
func (r savedKeyRow) bucketsCell() string {
	if r.Scope == config.ScopeFullAccess {
		return "all"
	}
	if len(r.Buckets) == 0 {
		return emptyCell
	}
	return truncateText(formatPerms(r.Buckets), 40)
}

// formatPerms renders a name→permission map as "a=rw b=readonly", sorted.
func formatPerms(perms map[string]string) string {
	ids := make([]string, 0, len(perms))
	for id := range perms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id+"="+perms[id])
	}
	return strings.Join(parts, " ")
}

// savedKeysByID indexes the active profile's keys by access key ID. Errors
// (no profile) yield an empty map so listings still work under LATITUDESH_TOKEN.
func savedKeysByID(cmd *cobra.Command) (map[string]string, string, map[string]config.StoredAccessKey) {
	_, profileName, p, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return map[string]string{}, "", map[string]config.StoredAccessKey{}
	}
	keys := p.ObjectStorageKeys()
	byID := make(map[string]string, len(keys))
	for name, k := range keys {
		if k.AccessKeyID != "" {
			byID[k.AccessKeyID] = name
		}
	}
	return byID, profileName, keys
}

// truncateText shortens s to max runes with an ellipsis.
func truncateText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// dash is the access-keys spelling of the shared table placeholder.
func dash(s string) string { return orEmptyCell(s) }

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// storageClassFlag validates --storage-class.
func storageClassFlag(cmd *cobra.Command) (string, error) {
	v, _ := cmd.Flags().GetString("storage-class")
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", objectstorage.ClassStandard, objectstorage.ClassHighPerformance:
		return v, nil
	case "hp", "high-performance", "highperformance":
		return objectstorage.ClassHighPerformance, nil
	}
	return "", exitcode.Errorf(exitcode.Usage, "invalid --storage-class %q (use standard or high_performance)", v)
}

// regionFlag returns --region upper-cased, rejecting AWS-looking values.
func regionFlag(cmd *cobra.Command) (string, error) {
	v, _ := cmd.Flags().GetString("region")
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if looksLikeAWSRegion(v) {
		return "", exitcode.Errorf(exitcode.Usage, "--region %q is not a Latitude site; use a site slug such as DAL, NYC or TYO4 (see 'lsh regions list')", v)
	}
	return strings.ToUpper(v), nil
}
