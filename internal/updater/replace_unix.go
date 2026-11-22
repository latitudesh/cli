//go:build !windows

package updater

// replace swaps staged into target with a single rename. Unix allows renaming
// over a running executable (the process keeps its open inode), so target is
// never missing, even if the update is interrupted.
func replace(target, staged string) error {
	return rename(staged, target)
}
