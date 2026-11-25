// Command sdkcoverage reports which latitudesh-go-sdk methods the lsh CLI
// exposes as commands, and fails when the SDK, the coverage manifest and the
// command tree disagree.
//
// Usage:
//
//	sdkcoverage report [-format markdown|text|json] [-sdk-dir DIR]
//	sdkcoverage check                 # exit 1 on any violation
//	sdkcoverage groups [-sdk-dir DIR] # dump the raw SDK surface
//	sdkcoverage commands              # dump the CLI's leaf command paths
//
// Everything runs offline against the module cache; no API token is involved.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/latitudesh/lsh/cmd"
	"github.com/latitudesh/lsh/internal/sdkcoverage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sdkcoverage:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	manifestPath := flags.String("manifest", sdkcoverage.ManifestFile, "path to the coverage manifest")
	sdkDir := flags.String("sdk-dir", "", "SDK source directory (defaults to the pinned module in the module cache)")
	format := flags.String("format", "markdown", "report format: markdown, text or json")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	switch command {
	case "groups":
		return runGroups(*sdkDir)
	case "commands":
		return runCommands()
	case "report":
		return runReport(*manifestPath, *sdkDir, *format)
	case "check":
		return runCheck(*manifestPath, *sdkDir)
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: sdkcoverage <report|check|groups|commands> [flags]

  report    render the coverage report (markdown by default)
  check     exit non-zero when the SDK, the manifest and the CLI disagree
  groups    dump the raw SDK service-group surface, for seeding the manifest
  commands  dump the CLI's leaf command paths

flags:
  -manifest path   coverage manifest (default: sdk-coverage.yaml in the working directory)
  -sdk-dir path    SDK source directory (default: pinned module in the module cache)
  -format fmt      markdown (default), text or json, for report
`)
}

func loadSurface(sdkDir string) (sdkcoverage.Surface, string, error) {
	if sdkDir == "" {
		dir, err := sdkcoverage.PinnedModuleDir()
		if err != nil {
			return sdkcoverage.Surface{}, "", err
		}
		sdkDir = dir
	}
	surface, err := sdkcoverage.ParseSurface(sdkDir)
	return surface, sdkDir, err
}

func leafCommands() ([]string, error) {
	root, err := cmd.Root()
	if err != nil {
		return nil, fmt.Errorf("building the command tree: %w", err)
	}
	return sdkcoverage.LeafCommands(root), nil
}

func reconcile(manifestPath, sdkDir string) (sdkcoverage.Report, string, error) {
	surface, dir, err := loadSurface(sdkDir)
	if err != nil {
		return sdkcoverage.Report{}, "", err
	}
	manifest, err := sdkcoverage.LoadManifest(manifestPath)
	if err != nil {
		return sdkcoverage.Report{}, "", err
	}
	leaves, err := leafCommands()
	if err != nil {
		return sdkcoverage.Report{}, "", err
	}
	return sdkcoverage.Reconcile(surface, manifest, leaves), sdkcoverage.VersionFromDir(dir), nil
}

func runGroups(sdkDir string) error {
	surface, dir, err := loadSurface(sdkDir)
	if err != nil {
		return err
	}
	fmt.Printf("# %s\n# %d groups, %d methods\n\n", dir, len(surface.Groups), surface.MethodCount())
	for _, name := range surface.Names() {
		group := surface.Groups[name]
		fmt.Println(name)
		for _, method := range group.Methods {
			fmt.Printf("  - %s\n", method)
		}
	}
	return nil
}

func runCommands() error {
	leaves, err := leafCommands()
	if err != nil {
		return err
	}
	for _, leaf := range leaves {
		fmt.Println(leaf)
	}
	return nil
}

func runReport(manifestPath, sdkDir, format string) error {
	report, version, err := reconcile(manifestPath, sdkDir)
	if err != nil {
		return err
	}
	switch format {
	case "markdown":
		fmt.Print(report.Markdown(version))
	case "text":
		fmt.Print(report.Text(version))
	case "json":
		data, err := report.JSON(version)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		return fmt.Errorf("unsupported report format %q (want markdown, text or json)", format)
	}
	return nil
}

func runCheck(manifestPath, sdkDir string) error {
	report, version, err := reconcile(manifestPath, sdkDir)
	if err != nil {
		return err
	}
	if !report.OK() {
		fmt.Fprintf(os.Stderr, "%d violation(s):\n", len(report.Violations))
		for _, v := range report.Violations {
			fmt.Fprintf(os.Stderr, "  - %s\n", v)
		}
		return fmt.Errorf("%s is out of sync with the SDK or the command tree", manifestPath)
	}
	c := report.Counts()
	fmt.Printf("ok: %s — %d/%d methods covered, %d excluded, %d pending; %d new top-level command groups\n",
		version, c.Covered, c.Methods, c.Excluded, c.Pending, c.NewCommandGroups)
	return nil
}
