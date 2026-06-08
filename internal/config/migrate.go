package config

import "encoding/json"

// legacyTopLevel is the previous single-token config shape that this
// CLI shipped before profile support. We detect it by parsing the raw
// JSON for an `Authorization` (or `authorization`) field at the top
// level and, if present, migrate it into a "default" profile.
type legacyTopLevel struct {
	AuthorizationA string `json:"Authorization"`
	AuthorizationB string `json:"authorization"`
	APIVersionA    string `json:"API-Version"`
	APIVersionB    string `json:"api-version"`
}

// migrateLegacyInto inspects raw bytes for the old top-level token and,
// if found and no profiles exist yet, materializes a "default" profile
// from it. Returns true when a migration was applied so the caller can
// persist the new format. Idempotent: a second load on a migrated file
// is a no-op and returns false.
func migrateLegacyInto(f *File, raw []byte) bool {
	if len(f.Profiles) > 0 {
		return false
	}
	var legacy legacyTopLevel
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return false
	}
	token := firstNonEmpty(legacy.AuthorizationA, legacy.AuthorizationB)
	if token == "" {
		return false
	}
	apiVersion := firstNonEmpty(legacy.APIVersionA, legacy.APIVersionB)
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	f.Profiles["default"] = Profile{
		Authorization: token,
		APIVersion:    apiVersion,
		Source:        SourceWithToken,
	}
	if f.DefaultProfile == "" {
		f.DefaultProfile = "default"
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
