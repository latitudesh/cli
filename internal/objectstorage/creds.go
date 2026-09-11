package objectstorage

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
)

// Environment variables understood by the S3 layer.
const (
	EnvAccessKeyID     = "LSH_S3_ACCESS_KEY_ID"
	EnvSecretAccessKey = "LSH_S3_SECRET_ACCESS_KEY"
	// EnvUseAWSEnv opts in to reading AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
	// as a fallback. Off by default: those variables almost always point at
	// the real AWS and would produce opaque 403s against Latitude.
	EnvUseAWSEnv     = "LSH_S3_USE_AWS_ENV"
	EnvEndpointURL   = "LSH_S3_ENDPOINT_URL"
	EnvSigningRegion = "LSH_S3_SIGNING_REGION"
)

// Credential is a resolved S3 credential plus where it came from. The secret
// is unexported so that formatting the value never prints it.
type Credential struct {
	// Name is the saved key name; empty for environment credentials.
	Name        string
	AccessKeyID string
	secret      string
	// Source describes the origin for messages and --debug:
	// "LSH_S3_ACCESS_KEY_ID (environment)", "saved key \"ci\" (profile acme)".
	Source string
	// FromEnv is true when the credential came from environment variables.
	FromEnv bool
	// Key carries the scope metadata of a saved key (zero for env).
	Key config.StoredAccessKey
	// Profile is the profile name the key was loaded from.
	Profile string
}

// NewCredential builds a credential from explicit values (tests, imports).
func NewCredential(id, secret, source string) Credential {
	return Credential{AccessKeyID: id, secret: secret, Source: source}
}

// Secret returns the secret access key.
func (c Credential) Secret() string { return c.secret }

// String never includes the secret.
func (c Credential) String() string {
	return fmt.Sprintf("access_key_id=%s source=%s secret=[redacted]", c.AccessKeyID, c.Source)
}

// GoString mirrors String for %#v.
func (c Credential) GoString() string { return c.String() }

// Describe renders the credential for user-facing messages, including its
// permission on bucketID when known.
func (c Credential) Describe(bucketID string) string {
	if c.FromEnv || c.Name == "" {
		return c.Source
	}
	perm := c.Key.Permission(bucketID)
	switch {
	case c.Key.Scope == config.ScopeFullAccess:
		return fmt.Sprintf("saved key %q (fullaccess)", c.Name)
	case perm != "":
		return fmt.Sprintf("saved key %q (limited_access, %s on this bucket)", c.Name, perm)
	default:
		return fmt.Sprintf("saved key %q (%s)", c.Name, c.Key.Scope)
	}
}

// CredentialOptions controls ResolveCredential.
type CredentialOptions struct {
	// AccessKeyName is the --access-key flag: force a saved key by name.
	AccessKeyName string
	// ProfileOverride is the --profile flag.
	ProfileOverride string
	// Write is true when the operation needs write permission on the bucket
	// (upload, delete, presign PUT).
	Write bool
}

// EnvCredential reads LSH_S3_* (and, when opted in, AWS_*) from the
// environment. ok is false when nothing is set; err reports a half-set pair.
func EnvCredential() (Credential, bool, error) {
	id, secret := os.Getenv(EnvAccessKeyID), os.Getenv(EnvSecretAccessKey)
	switch {
	case id != "" && secret != "":
		return Credential{AccessKeyID: id, secret: secret, Source: EnvAccessKeyID + " (environment)", FromEnv: true}, true, nil
	case id != "" && secret == "":
		return Credential{}, false, exitcode.Errorf(exitcode.Credentials, "%s is set but %s is missing", EnvAccessKeyID, EnvSecretAccessKey)
	case id == "" && secret != "":
		return Credential{}, false, exitcode.Errorf(exitcode.Credentials, "%s is set but %s is missing", EnvSecretAccessKey, EnvAccessKeyID)
	}
	if os.Getenv(EnvUseAWSEnv) == "1" || strings.EqualFold(os.Getenv(EnvUseAWSEnv), "true") {
		if os.Getenv("AWS_SESSION_TOKEN") != "" {
			// STS credentials are never Latitude access keys.
			return Credential{}, false, nil
		}
		aid, asecret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY")
		if aid != "" && asecret != "" {
			fmt.Fprintf(os.Stderr, "warning: using AWS_ACCESS_KEY_ID from the environment (%s=1)\n", EnvUseAWSEnv)
			return Credential{AccessKeyID: aid, secret: asecret, Source: "AWS_ACCESS_KEY_ID (environment, opt-in fallback)", FromEnv: true}, true, nil
		}
	}
	return Credential{}, false, nil
}

// ActiveProfile loads the config file and resolves the active profile
// (--profile > LSH_PROFILE > default_profile).
func ActiveProfile(override string) (*config.File, string, config.Profile, error) {
	f, err := config.Load()
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	name, p, err := f.Resolve(override)
	if err != nil {
		if errors.Is(err, config.ErrProfileNotFound) {
			if override != "" {
				return f, name, p, exitcode.Errorf(exitcode.Usage, "profile %q not found — run 'lsh profile list'", override)
			}
			return f, "", config.Profile{}, exitcode.Errorf(exitcode.Credentials, "no active profile — run 'lsh login' (or use LSH_S3_ACCESS_KEY_ID/LSH_S3_SECRET_ACCESS_KEY)")
		}
		return f, name, p, err
	}
	return f, name, p, nil
}

// ResolveCredential picks the credential for an operation on b:
//
//  1. LSH_S3_ACCESS_KEY_ID + LSH_S3_SECRET_ACCESS_KEY (environment)
//  2. --access-key <name> (saved key, must cover the bucket)
//  3. the least-privileged saved key that covers the bucket
//
// In endpoint-override mode only the environment is consulted: saved keys are
// never sent to a host the API did not vouch for.
func ResolveCredential(b *Bucket, o CredentialOptions) (Credential, error) {
	if c, ok, err := EnvCredential(); err != nil {
		return Credential{}, err
	} else if ok {
		return c, nil
	}
	if b != nil && b.EndpointOverride {
		return Credential{}, exitcode.Errorf(exitcode.Credentials,
			"saved access keys are not sent to custom endpoints; set %s and %s in the environment", EnvAccessKeyID, EnvSecretAccessKey)
	}
	_, profileName, profile, err := ActiveProfile(o.ProfileOverride)
	if err != nil {
		return Credential{}, err
	}
	keys := profile.ObjectStorageKeys()

	if o.AccessKeyName != "" {
		k, ok := keys[o.AccessKeyName]
		if !ok {
			return Credential{}, exitcode.Errorf(exitcode.NotFound, "no saved access key named %q in profile %s; run 'lsh s3 access-keys list --saved'", o.AccessKeyName, profileName)
		}
		c := fromStored(o.AccessKeyName, k, profileName)
		if b != nil && !b.EndpointOverride && b.ID != "" {
			// Covers() is true for every bucket ID on a fullaccess key, so
			// without this the class/site/project constraints that automatic
			// selection enforces would never be checked for an explicit key:
			// the request would go out and fail at the backend instead.
			if err := explainIncompatibleKey(o.AccessKeyName, k, b); err != nil {
				return Credential{}, err
			}
			if k.Scope != config.ScopeUnknown && !k.Covers(b.ID, false) {
				return Credential{}, exitcode.Errorf(exitcode.Permission, "saved key %q does not cover bucket %s", o.AccessKeyName, b.Display())
			}
			if o.Write && k.Scope == config.ScopeLimitedAccess && k.Permission(b.ID) != config.PermissionRW {
				return Credential{}, exitcode.Errorf(exitcode.Permission, "saved key %q is readonly on bucket %s; pick a key with rw permission (lsh s3 access-keys list) or create one: lsh s3 access-keys create --bucket %s --save", o.AccessKeyName, b.Display(), b.Name)
			}
		}
		return c, nil
	}

	if b == nil {
		return Credential{}, exitcode.Errorf(exitcode.Credentials, "no credential selected; pass --access-key <name> or set %s/%s", EnvAccessKeyID, EnvSecretAccessKey)
	}
	name, k, ok := SelectKey(keys, profile.DefaultObjectStorageKey(), b, o.Write)
	if !ok {
		return Credential{}, NoCredentialError(b, profileName, o.Write, keys)
	}
	return fromStored(name, k, profileName), nil
}

func fromStored(name string, k config.StoredAccessKey, profile string) Credential {
	return Credential{
		Name:        name,
		AccessKeyID: k.AccessKeyID,
		secret:      k.SecretAccessKey,
		Source:      fmt.Sprintf("saved key %q (profile %s)", name, profile),
		Key:         k,
		Profile:     profile,
	}
}

// explainIncompatibleKey reports why a saved key cannot serve b, or nil when
// nothing rules it out. It applies the constraints keyMatchesBucket uses for
// automatic selection — with the same "empty means unknown, so do not judge"
// rule, which keeps imported keys with partial metadata usable — but names the
// mismatch instead of silently skipping the key, because here the user picked
// it explicitly.
func explainIncompatibleKey(name string, k config.StoredAccessKey, b *Bucket) error {
	switch {
	case k.StorageClass != "" && b.StorageClass != "" && k.StorageClass != b.StorageClass:
		return exitcode.Errorf(exitcode.Usage,
			"saved key %q is a %s key and bucket %s is %s; the two backends do not share credentials — run 'lsh s3 access-keys list --saved' to pick another, or drop --access-key to let the CLI choose",
			name, k.StorageClass, b.Display(), b.StorageClass)
	case k.Site != "" && b.Site != "" && !strings.EqualFold(k.Site, b.Site):
		return exitcode.Errorf(exitcode.Usage,
			"saved key %q belongs to site %s and bucket %s is in %s; a %s key only works in its own site",
			name, strings.ToUpper(k.Site), b.Display(), strings.ToUpper(b.Site), ClassHighPerformance)
	case k.ProjectID != "" && b.ProjectID != "" && k.ProjectID != b.ProjectID:
		return exitcode.Errorf(exitcode.Usage,
			"saved key %q belongs to project %s and bucket %s to project %s",
			name, k.ProjectID, b.Display(), b.ProjectID)
	}
	return nil
}

// SelectKey applies the automatic selection rules over the saved keys:
// same storage class, compatible site and project, covering the bucket with
// the needed permission. Ranking: limited rw > limited readonly (reads only)
// > fullaccess; ties go to defaultKey, then to the newest key, then by name.
func SelectKey(keys map[string]config.StoredAccessKey, defaultKey string, b *Bucket, write bool) (string, config.StoredAccessKey, bool) {
	type cand struct {
		name string
		key  config.StoredAccessKey
		rank int
	}
	var cands []cand
	for name, k := range keys {
		if !keyMatchesBucket(k, b) {
			continue
		}
		if !k.Covers(b.ID, write) {
			continue
		}
		rank := 2
		if k.Scope == config.ScopeLimitedAccess {
			if k.Permission(b.ID) == config.PermissionRW {
				rank = 0
			} else {
				rank = 1
			}
		}
		cands = append(cands, cand{name, k, rank})
	}
	if len(cands) == 0 {
		return "", config.StoredAccessKey{}, false
	}
	sort.Slice(cands, func(i, j int) bool {
		a, c := cands[i], cands[j]
		if a.rank != c.rank {
			return a.rank < c.rank
		}
		if (a.name == defaultKey) != (c.name == defaultKey) {
			return a.name == defaultKey
		}
		if !a.key.CreatedAt.Equal(c.key.CreatedAt) {
			return a.key.CreatedAt.After(c.key.CreatedAt)
		}
		return a.name < c.name
	})
	return cands[0].name, cands[0].key, true
}

// keyMatchesBucket checks class, site and project compatibility. Empty values
// on either side are treated as wildcards so keys saved before the site was
// known keep working.
func keyMatchesBucket(k config.StoredAccessKey, b *Bucket) bool {
	if k.Scope == config.ScopeUnknown {
		return false
	}
	if k.StorageClass != "" && b.StorageClass != "" && k.StorageClass != b.StorageClass {
		return false
	}
	if k.Site != "" && b.Site != "" && !strings.EqualFold(k.Site, b.Site) {
		return false
	}
	if k.ProjectID != "" && b.ProjectID != "" && k.ProjectID != b.ProjectID {
		return false
	}
	return true
}

// NoCredentialError explains how to obtain a credential for b.
func NoCredentialError(b *Bucket, profile string, write bool, keys map[string]config.StoredAccessKey) error {
	what := "no saved S3 access key covers"
	if write {
		// Point out read-only keys that exist but cannot write.
		for name, k := range keys {
			if keyMatchesBucket(k, b) && k.Covers(b.ID, false) {
				what = fmt.Sprintf("saved key %q is readonly on this bucket; no saved key can write to", name)
				break
			}
		}
	}
	return exitcode.Errorf(exitcode.Credentials,
		"%s bucket %s (%s, profile %s).\n  create one:  lsh s3 access-keys create --bucket %s --save\n  or run:      lsh s3 configure\n  or export:   %s and %s",
		what, b.Display(), b.StorageClass, profile, b.Name, EnvAccessKeyID, EnvSecretAccessKey)
}

// SaveKey stores k under name in the active profile and persists the file.
//
// The read-modify-write runs inside config.Update so two processes saving keys
// at the same time cannot drop each other's entry: a lost entry would strand a
// live credential whose secret the API never returns again.
func SaveKey(profileOverride, name string, k config.StoredAccessKey) (string, error) {
	var profileName string
	err := config.Update(func(f *config.File) error {
		var p config.Profile
		var resolveErr error
		profileName, p, resolveErr = resolveProfileIn(f, profileOverride)
		if resolveErr != nil {
			return resolveErr
		}
		p.SetObjectStorageKey(name, k)
		f.SetProfile(profileName, p)
		return nil
	})
	return profileName, err
}

// resolveProfileIn resolves the active profile inside an already-loaded file,
// mapping the same errors ActiveProfile reports. It exists so a config.Update
// transaction works on the file it locked instead of loading a second copy.
func resolveProfileIn(f *config.File, override string) (string, config.Profile, error) {
	name, p, err := f.Resolve(override)
	if err != nil {
		if errors.Is(err, config.ErrProfileNotFound) {
			if override != "" {
				return name, p, exitcode.Errorf(exitcode.Usage, "profile %q not found — run 'lsh profile list'", override)
			}
			return "", config.Profile{}, exitcode.Errorf(exitcode.Credentials, "no active profile — run 'lsh login' (or use LSH_S3_ACCESS_KEY_ID/LSH_S3_SECRET_ACCESS_KEY)")
		}
		return name, p, err
	}
	return name, p, nil
}

// ForgetKey removes a saved key by name from the active profile.
func ForgetKey(profileOverride, name string) (string, bool, error) {
	var profileName string
	var removed bool
	err := config.Update(func(f *config.File) error {
		var p config.Profile
		var resolveErr error
		profileName, p, resolveErr = resolveProfileIn(f, profileOverride)
		if resolveErr != nil {
			return resolveErr
		}
		removed = p.RemoveObjectStorageKey(name)
		if removed {
			f.SetProfile(profileName, p)
		}
		return nil
	})
	if err != nil {
		return profileName, false, err
	}
	return profileName, removed, nil
}

// ForgetKeyByID removes every saved key with the given access key ID.
func ForgetKeyByID(profileOverride, accessKeyID string) (string, []string, error) {
	var profileName string
	var removed []string
	err := config.Update(func(f *config.File) error {
		var p config.Profile
		var resolveErr error
		profileName, p, resolveErr = resolveProfileIn(f, profileOverride)
		if resolveErr != nil {
			return resolveErr
		}
		removed = nil
		for name, k := range p.ObjectStorageKeys() {
			if k.AccessKeyID == accessKeyID {
				p.RemoveObjectStorageKey(name)
				removed = append(removed, name)
			}
		}
		if len(removed) > 0 {
			f.SetProfile(profileName, p)
		}
		return nil
	})
	if err != nil {
		return profileName, nil, err
	}
	sort.Strings(removed)
	return profileName, removed, nil
}
