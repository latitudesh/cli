package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	OS   = runtime.GOOS
	ARCH = runtime.GOARCH
)

type UpdateFile struct {
	Name       string
	Dir        string
	Extension  string
	Executable string
}

func NewUpdateFile() *UpdateFile {
	dir := buildDirName()
	ext := buildExtesion(OS)
	exec := buildExecutableName(OS)

	return &UpdateFile{
		Dir:        buildDirName(),
		Extension:  ext,
		Name:       dir + ext,
		Executable: exec,
	}
}

func (uf *UpdateFile) Url(version string) string {
	return fmt.Sprintf("https://github.com/latitudesh/lsh/releases/download/%s/%s", version, uf.Name)
}

func (uf *UpdateFile) Download(dstPath, version string) error {
	return downloadFile(uf.Url(version), dstPath)
}

func downloadFile(url, dstPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected HTTP status %s", url, resp.Status)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// buildDirName builds the correct file name for your Operating system and architecture
func buildDirName() string {
	caser := cases.Title(language.English)
	os := caser.String(OS)

	arch := parseArch(ARCH)

	return fmt.Sprintf("lsh_%s_%s", os, arch)
}

func buildExtesion(os string) string {
	if os == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

func buildExecutableName(os string) string {
	if os == "windows" {
		return "lsh.exe"
	}
	return "lsh"
}

// parseArch expects a GOARCH and returns the arch in our github release format
func parseArch(goArch string) string {
	if goArch == "amd64" {
		return "x86_64"
	}
	if goArch == "386" {
		return "i386"
	}

	return goArch
}
