package sdkcoverage

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// SDKModulePath is the Go module this package reconciles against.
const SDKModulePath = "github.com/latitudesh/latitudesh-go-sdk"

// PinnedModuleDir resolves the local source directory of the SDK version this
// repo requires, so the surface can be parsed straight from the module cache
// with no network access.
func PinnedModuleDir() (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", SDKModulePath).Output()
	if err != nil {
		return "", fmt.Errorf("sdkcoverage: resolving %s: %w", SDKModulePath, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("sdkcoverage: %s has no local directory (run `go mod download`)", SDKModulePath)
	}
	return dir, nil
}

// VersionFromDir derives the version string reports display from the SDK
// source directory. Module-cache directories are named module@version, so the
// suffix after "@" is kept only when it looks like a Go module version. Any
// other basename (a hand-picked -sdk-dir) passes through whole.
func VersionFromDir(dir string) string {
	base := filepath.Base(dir)
	if at := strings.LastIndex(base, "@"); at >= 0 {
		if v := base[at+1:]; len(v) > 1 && v[0] == 'v' && v[1] >= '0' && v[1] <= '9' {
			return v
		}
	}
	return base
}
