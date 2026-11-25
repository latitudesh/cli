package cmd

import (
	"path/filepath"
	"testing"

	"github.com/latitudesh/lsh/internal/sdkcoverage"
)

// TestCLISDKCoverage keeps sdk-coverage.yaml honest about the gap between what
// latitudesh-go-sdk exposes and what this CLI ships.
//
// It is a static check: no network, no API token. It fails only on
// contradictions: a manifest path the CLI does not register, a method or group
// the SDK no longer exposes, a leaf command no entry claims, or a half-written
// exclusion. An SDK bump that adds methods is NOT a failure: a pending method is
// what triggers scaffolding, and failing would block every bump on a product
// decision its author was never asked to make. Pending work surfaces in
// `go run ./cmd/sdkcoverage report` instead.
func TestCLISDKCoverage(t *testing.T) {
	sdkDir, err := sdkcoverage.PinnedModuleDir()
	if err != nil {
		t.Fatalf("resolving the pinned SDK module: %v", err)
	}
	surface, err := sdkcoverage.ParseSurface(sdkDir)
	if err != nil {
		t.Fatalf("parsing the SDK surface at %s: %v", sdkDir, err)
	}

	manifestPath := filepath.Join("..", sdkcoverage.ManifestFile)
	manifest, err := sdkcoverage.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loading %s: %v", manifestPath, err)
	}

	root, err := Root()
	if err != nil {
		t.Fatalf("building the command tree: %v", err)
	}
	leaves := sdkcoverage.LeafCommands(root)
	if len(leaves) == 0 {
		t.Fatal("the CLI registered no commands; tree introspection is broken")
	}

	report := sdkcoverage.Reconcile(surface, manifest, leaves)
	for _, v := range report.Violations {
		t.Errorf("%s", v)
	}
	if !report.OK() {
		t.Logf("%d SDK group(s) parsed from %s; %d leaf command(s) registered", len(surface.Groups), sdkDir, len(leaves))
		t.Log("fix the mismatch by updating " + sdkcoverage.ManifestFile + " (see its header comment)")
	}
}
