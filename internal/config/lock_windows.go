//go:build windows

package config

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// lockConfig takes an exclusive lock on path, blocking until it is available.
// Windows releases the range lock when the handle closes, matching the flock
// behaviour used on the other platforms.
func lockConfig(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		return nil, fmt.Errorf("config: open lock: %w", err)
	}
	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		f.Close()
		return nil, fmt.Errorf("config: lock: %w", err)
	}
	return func() {
		var release windows.Overlapped
		_ = windows.UnlockFileEx(handle, 0, 1, 0, &release)
		_ = f.Close()
	}, nil
}
