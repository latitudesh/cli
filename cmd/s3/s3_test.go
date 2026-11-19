package s3

import (
	"testing"

	"github.com/latitudesh/lsh/internal/objectstorage"
)

// TestUsesSavedCredential guards the condition that decides whether a bucket's
// site has to be resolved: it only matters when the credential comes from the
// profile, since LSH_S3_* bypasses selection entirely.
func TestUsesSavedCredential(t *testing.T) {
	t.Setenv(objectstorage.EnvAccessKeyID, "")
	t.Setenv(objectstorage.EnvSecretAccessKey, "")
	if !usesSavedCredential() {
		t.Error("without environment credentials the profile is used")
	}
	t.Setenv(objectstorage.EnvAccessKeyID, "AK")
	t.Setenv(objectstorage.EnvSecretAccessKey, "SK")
	if usesSavedCredential() {
		t.Error("environment credentials bypass the profile, so the site is irrelevant")
	}
}
