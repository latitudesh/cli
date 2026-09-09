package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Update applies mutate to the configuration file as one transaction: the file
// is locked, loaded, mutated and written back before the lock is released.
//
// Load/Save on their own are not enough. Save writes a temp file and renames it
// atomically, so a single write can never be torn — but two processes that both
// load, mutate and save (two `lsh s3 access-keys create --save` runs, say) each
// hold a full snapshot, and the second rename silently drops the first one's
// change. For an S3 access key that means losing a secret the API only ever
// returns once, while the credential stays live on the server. Serialising the
// whole read-modify-write is the only way to avoid it.
//
// The lock is advisory and per-file; readers do not take it, so a concurrent
// reader may still observe the pre-update file (which is consistent, thanks to
// the atomic rename).
func Update(mutate func(*File) error) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	unlock, err := lockConfig(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	f, err := Load()
	if err != nil {
		return err
	}
	if err := mutate(f); err != nil {
		return err
	}
	return Save(f)
}
