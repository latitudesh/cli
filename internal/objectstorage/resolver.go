package objectstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/minio/minio-go/v7/pkg/s3utils"
	"github.com/spf13/viper"
)

// Storage classes as reported by the API.
const (
	ClassStandard        = "standard"
	ClassHighPerformance = "high_performance"
)

// Bucket is the resolved view of an object storage bucket: everything the S3
// layer needs (endpoint, real bucket name, signing region) plus the metadata
// shown by `stat` and used for safety checks (versioning, object lock).
type Bucket struct {
	ID           string
	Name         string // display name
	BucketName   string // real name on the S3 endpoint
	Endpoint     string // https://…
	StorageClass string
	// Site is the Latitude site slug (DAL, TYO4…). The SDK model drops it, so
	// it is filled lazily by Resolver.FillSite when needed.
	Site        string
	City        string
	Country     string
	ProjectID   string
	ProjectSlug string
	ProjectName string
	Versioning  bool
	Locking     bool
	// RetentionMode is NONE, GOVERNANCE or COMPLIANCE when object lock is on.
	RetentionMode string
	RetentionDays int64
	// Source is "default" (created through the API) or "synchronized"
	// (imported from the provider; may lack an endpoint).
	Source        string
	CreatedAt     *time.Time
	SigningRegion string
	// EndpointOverride is true when the bucket was not looked up in the API
	// (--endpoint-url / LSH_S3_ENDPOINT_URL): only BucketName, Endpoint and
	// SigningRegion are meaningful.
	EndpointOverride bool
	// Data keeps the SDK payload for table/JSON rendering.
	Data components.ObjectStorageData
}

// ProjectRef returns the best human identifier for the bucket's project.
func (b *Bucket) ProjectRef() string {
	if b.ProjectSlug != "" {
		return b.ProjectSlug
	}
	return b.ProjectID
}

// Display returns "name (bkt_…)" for messages.
func (b *Bucket) Display() string {
	if b.EndpointOverride {
		return b.BucketName
	}
	if b.ID == "" {
		return b.Name
	}
	return fmt.Sprintf("%s (%s)", b.Name, b.ID)
}

// Validate makes sure the bucket can be addressed over S3.
func (b *Bucket) Validate() error {
	if b.BucketName == "" {
		return exitcode.Errorf(exitcode.Generic, "bucket %s has no backend bucket name yet (it may still be provisioning); retry in a few seconds", b.Display())
	}
	if b.Endpoint == "" {
		return exitcode.Errorf(exitcode.Generic, "bucket %s has no S3 endpoint (source: %s); pass --endpoint-url if you know where it lives", b.Display(), b.Source)
	}
	// Validate up front so an invalid backend name is a usage error (exit 2)
	// on every command instead of surfacing as a generic minio error.
	if err := s3utils.CheckValidBucketName(b.BucketName); err != nil {
		return exitcode.Errorf(exitcode.Usage, "invalid bucket name %q: %v", b.BucketName, err)
	}
	return nil
}

// BucketFromData converts the SDK model into a Bucket.
func BucketFromData(d components.ObjectStorageData) *Bucket {
	b := &Bucket{Data: d}
	if d.ID != nil {
		b.ID = *d.ID
	}
	a := d.Attributes
	if a == nil {
		return b
	}
	b.Name = str(a.Name)
	b.BucketName = str(a.BucketName)
	b.Endpoint = str(a.Endpoint)
	if a.StorageClass != nil {
		b.StorageClass = string(*a.StorageClass)
	}
	if a.Versioning != nil {
		b.Versioning = *a.Versioning
	}
	if a.Locking != nil {
		b.Locking = *a.Locking
	}
	if a.RetentionMode != nil {
		b.RetentionMode = string(*a.RetentionMode)
	}
	if a.RetentionPeriod != nil {
		b.RetentionDays = *a.RetentionPeriod
	}
	b.Source = str(a.Source)
	b.CreatedAt = a.CreatedAt
	if a.Region != nil {
		b.Site = str(a.Region.ID)
		b.City = str(a.Region.City)
		b.Country = str(a.Region.Country)
	}
	if a.Project != nil {
		b.ProjectID = str(a.Project.ID)
		b.ProjectSlug = str(a.Project.Slug)
		b.ProjectName = str(a.Project.Name)
	}
	b.SigningRegion = SigningRegion(b.Endpoint)
	return b
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Resolver turns a bucket token (display name, bkt_ ID or backend name) into
// a Bucket using the Latitude API, or bypasses the API when an endpoint
// override is in effect.
type Resolver struct {
	API *sdk.Latitudesh
	// Project narrows name lookups to one project (ID or slug). Empty means
	// the whole team.
	Project string
	// EndpointURL enables the API-less mode: the token is the real bucket name
	// on that endpoint. Saved credentials are never used in this mode.
	EndpointURL string
	// SigningRegion overrides the region derived from the endpoint.
	SigningRegion string
	// ClassFilter, when set, keeps only buckets of this storage class when a
	// name matches several (already normalized to standard|high_performance).
	ClassFilter string
	// SiteFilter, when set, keeps only buckets in this Latitude site (slug,
	// e.g. DAL, TYO4) when a name matches several.
	SiteFilter string
	// HasFilterFlags reports whether the command registered
	// --storage-class/-c and --site. The ambiguity error only advertises them
	// when it does; cp/mv/sync spend --storage-class on a different meaning.
	HasFilterFlags bool
	// FilterErr, when set, is returned by Resolve/ListBuckets before any call
	// (e.g. an invalid --storage-class value).
	FilterErr error
	// RetryOptions are passed to every SDK call.
	RetryOptions []operations.Option
}

// Resolve resolves ref.Bucket.
func (r *Resolver) Resolve(ctx context.Context, token string) (*Bucket, error) {
	if r.FilterErr != nil {
		return nil, r.FilterErr
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "missing bucket name")
	}
	if r.EndpointURL != "" {
		b := &Bucket{
			Name:             token,
			BucketName:       token,
			Endpoint:         r.EndpointURL,
			EndpointOverride: true,
			SigningRegion:    SigningRegion(r.EndpointURL),
		}
		if r.SigningRegion != "" {
			b.SigningRegion = r.SigningRegion
		}
		return b, nil
	}
	if r.API == nil {
		return nil, exitcode.Errorf(exitcode.Credentials, "not logged in — run 'lsh login', or set LSH_S3_ENDPOINT_URL to address the bucket without the API")
	}

	var b *Bucket
	if strings.HasPrefix(token, "bkt_") {
		resp, err := r.API.ObjectStorage.GetStorageBucket(ctx, token, r.RetryOptions...)
		if err != nil {
			return nil, humanizeAPIError(err, fmt.Sprintf("bucket %q", token))
		}
		if resp.Object == nil || resp.Object.Data == nil {
			return nil, bucketNotFound(token, r.Project)
		}
		b = BucketFromData(*resp.Object.Data)
	} else {
		list, err := r.ListBuckets(ctx)
		if err != nil {
			return nil, err
		}
		var matches []components.ObjectStorageData
		for _, d := range list {
			if d.Attributes == nil {
				continue
			}
			if str(d.Attributes.Name) == token || str(d.Attributes.BucketName) == token {
				matches = append(matches, d)
			}
		}
		nameMatches := len(matches)
		matches, err = r.applyFilters(ctx, matches)
		if err != nil {
			return nil, err
		}
		switch len(matches) {
		case 0:
			if nameMatches > 0 {
				// The name existed but the --storage-class/--site filters ruled
				// every candidate out.
				return nil, exitcode.Errorf(exitcode.NotFound, "no bucket named %q matches the given filters (%s); run 'lsh s3 ls' to see the buckets", token, r.filterDesc())
			}
			return nil, bucketNotFound(token, r.Project)
		case 1:
			b = BucketFromData(matches[0])
		default:
			return nil, ambiguousBucket(token, matches, r.HasFilterFlags)
		}
		// The list payload omits some attributes; fetch the full record.
		if b.ID != "" && (b.Endpoint == "" || b.BucketName == "") {
			resp, err := r.API.ObjectStorage.GetStorageBucket(ctx, b.ID, r.RetryOptions...)
			if err == nil && resp.Object != nil && resp.Object.Data != nil {
				b = BucketFromData(*resp.Object.Data)
			}
		}
	}
	if r.SigningRegion != "" {
		b.SigningRegion = r.SigningRegion
	}
	return b, nil
}

// ListBuckets returns the team's buckets (filtered by r.Project when set).
func (r *Resolver) ListBuckets(ctx context.Context) ([]components.ObjectStorageData, error) {
	if r.FilterErr != nil {
		return nil, r.FilterErr
	}
	if r.API == nil {
		return nil, exitcode.Errorf(exitcode.Credentials, "not logged in — run 'lsh login' first")
	}
	var filter *string
	if r.Project != "" {
		p := r.Project
		filter = &p
	}
	resp, err := r.API.ObjectStorage.GetStorageBuckets(ctx, filter, r.RetryOptions...)
	if err != nil {
		return nil, humanizeAPIError(err, "buckets")
	}
	if resp.ObjectStorages == nil {
		return nil, nil
	}
	return resp.ObjectStorages.Data, nil
}

// applyFilters narrows name matches by storage class and site. Class comes
// straight from the list payload; the site slug does not (the SDK model drops
// it), so it is fetched once from the API only when a site filter is set.
func (r *Resolver) applyFilters(ctx context.Context, matches []components.ObjectStorageData) ([]components.ObjectStorageData, error) {
	if (r.ClassFilter == "" && r.SiteFilter == "") || len(matches) == 0 {
		return matches, nil
	}
	var sites map[string]string
	if r.SiteFilter != "" {
		var err error
		if sites, err = RawBucketSitesForProject(ctx, "", r.Project); err != nil {
			return nil, err
		}
	}
	out := make([]components.ObjectStorageData, 0, len(matches))
	for _, d := range matches {
		b := BucketFromData(d)
		if r.ClassFilter != "" && !strings.EqualFold(b.StorageClass, r.ClassFilter) {
			continue
		}
		if r.SiteFilter != "" {
			site := b.Site
			if site == "" && d.ID != nil {
				site = sites[*d.ID]
			}
			if !strings.EqualFold(site, r.SiteFilter) {
				continue
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// filterDesc renders the active filters for error messages.
func (r *Resolver) filterDesc() string {
	var parts []string
	if r.ClassFilter != "" {
		parts = append(parts, "--storage-class "+r.ClassFilter)
	}
	if r.SiteFilter != "" {
		parts = append(parts, "--site "+r.SiteFilter)
	}
	return strings.Join(parts, ", ")
}

func ambiguousBucket(token string, matches []components.ObjectStorageData, hasFilterFlags bool) error {
	lines := make([]string, 0, len(matches))
	for _, d := range matches {
		b := BucketFromData(d)
		lines = append(lines, fmt.Sprintf("  %s  project=%s  class=%s  site=%s", b.ID, b.ProjectRef(), b.StorageClass, firstNonEmpty(b.Site, b.City)))
	}
	sort.Strings(lines)
	narrow := "narrow it with --project, or use the bkt_ ID"
	if hasFilterFlags {
		narrow = "narrow it with --project, --storage-class/-c or --site, or use the bkt_ ID"
	}
	return exitcode.Errorf(exitcode.Usage, "bucket name %q is ambiguous:\n%s\n%s", token, strings.Join(lines, "\n"), narrow)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// FillSite fetches the bucket's site slug (e.g. TYO4) straight from the API
// when the SDK model did not carry it. It is a no-op when already known or in
// endpoint-override mode.
func (r *Resolver) FillSite(ctx context.Context, b *Bucket) error {
	if b == nil || b.Site != "" || b.EndpointOverride || b.ID == "" {
		return nil
	}
	sites, err := RawBucketSites(ctx, b.ID)
	if err != nil {
		return err
	}
	b.Site = sites[b.ID]
	return nil
}

// RawBucketSites returns bucket ID → site slug for one bucket (bucketID set)
// or for every bucket of the team (bucketID empty). It talks to the API
// directly because the generated model discards `region.site`.
func RawBucketSites(ctx context.Context, bucketID string) (map[string]string, error) {
	return RawBucketSitesForProject(ctx, bucketID, "")
}

// RawBucketSitesForProject is RawBucketSites restricted to one project (ID or
// slug) when bucketID is empty, avoiding a team-wide listing.
func RawBucketSitesForProject(ctx context.Context, bucketID, project string) (map[string]string, error) {
	path := "/storage/buckets"
	var query url.Values
	if bucketID != "" {
		path += "/" + url.PathEscape(bucketID)
	} else if project != "" {
		query = url.Values{"filter[project]": {project}}
	}
	body, err := rawAPIGet(ctx, path, query)
	if err != nil {
		return nil, err
	}
	type envelope struct {
		Data json.RawMessage `json:"data"`
	}
	type item struct {
		ID         string `json:"id"`
		Attributes struct {
			Region struct {
				Site struct {
					Slug string `json:"slug"`
				} `json:"site"`
			} `json:"region"`
		} `json:"attributes"`
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("could not parse bucket response: %w", err)
	}
	out := map[string]string{}
	var many []item
	if err := json.Unmarshal(env.Data, &many); err == nil {
		for _, it := range many {
			out[it.ID] = it.Attributes.Region.Site.Slug
		}
		return out, nil
	}
	var one item
	if err := json.Unmarshal(env.Data, &one); err != nil {
		return nil, fmt.Errorf("could not parse bucket response: %w", err)
	}
	out[one.ID] = one.Attributes.Region.Site.Slug
	return out, nil
}

// RawAccessKeyScopes returns, per access key ID, the bucket names and access
// level the API reports (`buckets[]` and `access`), which the SDK model does
// not expose. project is the project ID or slug.
func RawAccessKeyScopes(ctx context.Context, project string) (map[string]AccessKeyScope, error) {
	body, err := rawAPIGet(ctx, "/storage/access_keys", url.Values{"project": {project}})
	if err != nil {
		return nil, err
	}
	var env struct {
		Data map[string][]struct {
			AccessKeyID string   `json:"access_key_id"`
			Username    string   `json:"username"`
			Access      string   `json:"access"`
			Buckets     []string `json:"buckets"`
			Region      string   `json:"region"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("could not parse access keys response: %w", err)
	}
	out := map[string]AccessKeyScope{}
	for class, keys := range env.Data {
		for _, k := range keys {
			out[k.AccessKeyID] = AccessKeyScope{Access: k.Access, Buckets: k.Buckets, StorageClass: class, Site: k.Region, Username: k.Username}
		}
	}
	return out, nil
}

// AccessKeyScope is the scope information the API returns for a key.
type AccessKeyScope struct {
	// Access is fullaccess, rw, readonly or "" (unknown).
	Access       string
	Buckets      []string // backend bucket names
	StorageClass string
	Site         string
	Username     string
}

// rawAPIGet performs an authenticated GET against the Latitude API and returns
// the body. It reuses the same connection settings (hostname, scheme, token,
// API version) the SDK client is configured with.
func rawAPIGet(ctx context.Context, path string, query url.Values) ([]byte, error) {
	token := viper.GetString("Authorization")
	if token == "" {
		return nil, exitcode.Errorf(exitcode.Credentials, "not logged in — run 'lsh login' first")
	}
	host := viper.GetString("hostname")
	if host == "" {
		host = "api.latitude.sh"
	}
	scheme := viper.GetString("scheme")
	if scheme == "" {
		scheme = "https"
	}
	u := scheme + "://" + host + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(token, "Bearer "))
	req.Header.Set("Accept", "application/vnd.api+json, application/json")
	req.Header.Set("User-Agent", "lsh/"+version.Version)
	if v := viper.GetString("api-version"); v != "" {
		req.Header.Set("API-Version", v)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exitcode.Errorf(exitcode.Generic, "could not reach %s: %v", host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, humanizeAPIStatus(resp.StatusCode, body, path)
	}
	return body, nil
}
