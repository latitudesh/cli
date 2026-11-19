package config

import (
	"sort"
	"time"
)

// Access-key scopes as reported by the API.
const (
	ScopeFullAccess    = "fullaccess"
	ScopeLimitedAccess = "limited_access"
	// ScopeUnknown marks keys imported without being matched against the
	// API. They are only used when selected explicitly with --access-key.
	ScopeUnknown = "unknown"
)

// Per-bucket permissions of a limited_access key.
const (
	PermissionRW       = "rw"
	PermissionReadOnly = "readonly"
)

// Origins of a stored key (informational).
const (
	KeySourceConfigure = "configure"
	KeySourceCreate    = "access-key-create"
	KeySourceMakeBkt   = "mb"
	KeySourceImport    = "import"
)

// ObjectStorageConfig is the per-profile store of S3 access keys.
type ObjectStorageConfig struct {
	// DefaultKey breaks ties when several saved keys cover a bucket equally.
	DefaultKey string `json:"default_key,omitempty"`
	// Keys is indexed by the logical key name the user passes to --access-key.
	Keys map[string]StoredAccessKey `json:"keys,omitempty"`
}

// StoredAccessKey is one S3 access key saved locally. The scope metadata is
// what lets the CLI pick the least-privileged key covering a bucket without
// asking the API again.
type StoredAccessKey struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	// StorageClass is standard (Wasabi) or high_performance (VAST).
	StorageClass string `json:"storage_class"`
	// Site is the Latitude site slug (e.g. TYO4). Set for high_performance
	// keys, which are bound to one VAST cluster; empty for standard keys,
	// which cover every site of the project.
	Site      string `json:"site,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	// Scope is fullaccess, limited_access or unknown.
	Scope string `json:"scope"`
	// Buckets maps bkt_ IDs to rw|readonly for limited_access keys.
	Buckets map[string]string `json:"buckets,omitempty"`
	// Username is the backend identity the API needs on DELETE.
	Username  string    `json:"username,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// String renders the key without its secret so accidental %v/%s formatting
// never leaks it into logs or errors.
func (k StoredAccessKey) String() string {
	return "access_key_id=" + k.AccessKeyID + " secret=[redacted]"
}

// GoString mirrors String for %#v.
func (k StoredAccessKey) GoString() string { return k.String() }

// Covers reports whether the key grants access to bucketID with the
// requested permission ("rw" or "readonly"); a fullaccess key covers every
// bucket, a limited key only the listed ones.
func (k StoredAccessKey) Covers(bucketID string, needWrite bool) bool {
	switch k.Scope {
	case ScopeFullAccess:
		return true
	case ScopeLimitedAccess:
		perm, ok := k.Buckets[bucketID]
		if !ok {
			return false
		}
		return !needWrite || perm == PermissionRW
	}
	return false
}

// Permission returns rw, readonly or "" for bucketID.
func (k StoredAccessKey) Permission(bucketID string) string {
	if k.Scope == ScopeFullAccess {
		return PermissionRW
	}
	return k.Buckets[bucketID]
}

// ObjectStorageKeys returns the saved keys of the profile (never nil).
func (p Profile) ObjectStorageKeys() map[string]StoredAccessKey {
	if p.ObjectStorage == nil || p.ObjectStorage.Keys == nil {
		return map[string]StoredAccessKey{}
	}
	return p.ObjectStorage.Keys
}

// SortedObjectStorageKeyNames returns the saved key names alphabetically.
func (p Profile) SortedObjectStorageKeyNames() []string {
	keys := p.ObjectStorageKeys()
	names := make([]string, 0, len(keys))
	for n := range keys {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SetObjectStorageKey inserts or replaces a saved key.
func (p *Profile) SetObjectStorageKey(name string, k StoredAccessKey) {
	if p.ObjectStorage == nil {
		p.ObjectStorage = &ObjectStorageConfig{}
	}
	if p.ObjectStorage.Keys == nil {
		p.ObjectStorage.Keys = map[string]StoredAccessKey{}
	}
	p.ObjectStorage.Keys[name] = k
}

// RemoveObjectStorageKey deletes a saved key by name, clearing DefaultKey if
// it pointed at it. Returns false when the name was not stored.
func (p *Profile) RemoveObjectStorageKey(name string) bool {
	if p.ObjectStorage == nil || p.ObjectStorage.Keys == nil {
		return false
	}
	if _, ok := p.ObjectStorage.Keys[name]; !ok {
		return false
	}
	delete(p.ObjectStorage.Keys, name)
	if p.ObjectStorage.DefaultKey == name {
		p.ObjectStorage.DefaultKey = ""
	}
	return true
}

// FindObjectStorageKeyByID looks a saved key up by its access key ID.
func (p Profile) FindObjectStorageKeyByID(accessKeyID string) (string, StoredAccessKey, bool) {
	for name, k := range p.ObjectStorageKeys() {
		if k.AccessKeyID == accessKeyID {
			return name, k, true
		}
	}
	return "", StoredAccessKey{}, false
}

// DefaultObjectStorageKey returns the configured tie-breaker key name.
func (p Profile) DefaultObjectStorageKey() string {
	if p.ObjectStorage == nil {
		return ""
	}
	return p.ObjectStorage.DefaultKey
}
