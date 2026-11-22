package updater

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/latitudesh/lsh/internal/utils"
)

// verifyTimeout bounds the `--version` smoke test run against the new binary.
const verifyTimeout = 30 * time.Second

// rename is os.Rename, swappable in tests to simulate a failed swap.
var rename = os.Rename

// Update expects a version string and updates the cli to it.
//
// The running binary is only touched after the new one has been staged in the
// same directory and has reported the expected version, so a failed update
// leaves the current install in place.
func Update(version string) error {
	log.Println("Update started!")

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running lsh binary: %w", err)
	}
	target, err := resolveExecutable(exe)
	if err != nil {
		return err
	}

	updateFile := NewUpdateFile()

	tempDir, err := os.MkdirTemp("", "lsh-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	destinationPath := filepath.Join(tempDir, updateFile.Name)

	log.Println("Downloading new version...")
	err = updateFile.Download(destinationPath, version)
	if err != nil {
		return err
	}
	log.Println("Download finished successfully!")

	err = decompress(tempDir, updateFile)
	if err != nil {
		return err
	}

	newExecPath := filepath.Join(tempDir, updateFile.Dir, updateFile.Executable)
	staged, err := stage(newExecPath, target)
	if err != nil {
		return err
	}
	// No-op after a successful swap, since the staged file has been renamed.
	defer os.Remove(staged)

	if err := verifyVersion(staged, version); err != nil {
		return err
	}

	if err := replace(target, staged); err != nil {
		return fmt.Errorf("install new binary at %s: %w", target, err)
	}

	log.Println("Update finished successfully!")

	return nil
}

// resolveExecutable follows symlinks so the update replaces the installed
// binary rather than a link to it. os.Executable does not resolve symlinks on
// macOS, where install.sh links /usr/local/bin/lsh to ~/.lsh/lsh.
func resolveExecutable(exe string) (string, error) {
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve the running lsh binary %s: %w", exe, err)
	}
	return resolved, nil
}

// stage copies the new binary next to target, so the final swap is a rename
// within one filesystem. Renaming straight out of the temp dir fails with
// "invalid cross-device link" when /tmp is a separate filesystem (e.g. tmpfs).
func stage(src, target string) (string, error) {
	dir := filepath.Dir(target)

	mode := os.FileMode(0o755)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}

	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	// Keep the target's extension: Windows only runs files ending in .exe.
	out, err := os.CreateTemp(dir, ".lsh-update-*"+filepath.Ext(target))
	if err != nil {
		return "", fmt.Errorf("prepare update in %s: %w", dir, err)
	}
	staged := out.Name()

	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(staged, mode)
	}
	if err != nil {
		os.Remove(staged)
		return "", fmt.Errorf("prepare update in %s: %w", dir, err)
	}

	return staged, nil
}

// verifyVersion runs the staged binary with --version and checks that it
// reports the release being installed.
func verifyVersion(path, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return fmt.Errorf("new binary failed to run, keeping the current one: %w", err)
	}
	if !slices.Contains(strings.Fields(string(out)), version) {
		return fmt.Errorf("new binary reports %q, expected %s; keeping the current one", strings.TrimSpace(string(out)), version)
	}
	return nil
}

// swapWithBackup moves target to backup, then staged to target. If the second
// rename fails, the original binary is moved back so target never stays empty.
func swapWithBackup(target, staged, backup string) error {
	if err := rename(target, backup); err != nil {
		return err
	}
	if err := rename(staged, target); err != nil {
		if rerr := rename(backup, target); rerr != nil {
			return fmt.Errorf("%w; restoring the previous binary also failed (%v), it is at %s", err, rerr, backup)
		}
		return err
	}
	return nil
}

func decompress(destination string, file *UpdateFile) error {
	source := filepath.Join(destination, file.Name)
	if file.Extension == ".tar.gz" {
		err := utils.Untar(source, destination)
		if err != nil {
			return err
		}

		return nil
	}

	err := utils.Unzip(source, destination)
	if err != nil {
		return err
	}
	return nil
}
