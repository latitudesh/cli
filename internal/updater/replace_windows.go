//go:build windows

package updater

import "os"

// replace moves the running binary aside before installing the new one:
// Windows refuses to overwrite a running .exe but allows renaming it. The
// backup cannot be deleted while it runs, so it is removed on the next update.
func replace(target, staged string) error {
	backup := target + "-old"
	_ = os.Remove(backup)
	return swapWithBackup(target, staged, backup)
}
