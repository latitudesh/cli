//go:build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

// lockConfig takes an exclusive advisory lock on path, blocking until it is
// available. The kernel releases flock locks when the file descriptor closes,
// so a process that dies mid-update never leaves the lock behind.
func lockConfig(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		return nil, fmt.Errorf("config: open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("config: lock: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
