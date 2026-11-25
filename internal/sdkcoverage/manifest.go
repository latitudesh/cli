package sdkcoverage

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ManifestFile is the manifest's conventional name at the repository root.
const ManifestFile = "sdk-coverage.yaml"

// SupportedManifestVersion is the only manifest schema version this code
// understands. Bump it together with any breaking change to the layout below.
const SupportedManifestVersion = 1

// Reason explains why an SDK method is deliberately not exposed as a command.
// It decides whether the exclusion should ever expire: an API limitation can
// disappear when the API grows, while a product decision does not.
type Reason string

const (
	// ReasonAPIConstraint means the API does not offer enough to build a usable
	// command (a stub, a broken endpoint, a response the SDK cannot decode).
	ReasonAPIConstraint Reason = "api_constraint"

	// ReasonProductDecision means the team does not want this on the command
	// line, whatever the API offers.
	ReasonProductDecision Reason = "product_decision"

	// ReasonDeprecated means the upstream endpoint is going away.
	ReasonDeprecated Reason = "deprecated"
)

// Exclusion records why a method has no command.
type Exclusion struct {
	Reason Reason `yaml:"reason"`
	Notes  string `yaml:"notes,omitempty"`
}

// Paths is one or more command paths. A method usually backs one subcommand,
// but the mapping is many-to-many: BlockStorage.ListVolumes backs both
// `volume list` and `volume get`, and `servers reinstall` reads Servers.Get
// when it waits. The YAML form accepts a scalar or a list.
type Paths []string

func (p *Paths) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*p = Paths{s}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*p = Paths(list)
		return nil
	}
	return fmt.Errorf("line %d: expected a command path or a list of paths", node.Line)
}

// GroupEntry is the manifest record for one SDK service group.
//
// Coverage is per method: a method listed under Commands is covered by those
// commands, a method under Excluded was looked at and deliberately left out,
// and a method in neither is pending. A group with no entry at all is a new
// command group waiting to be scaffolded, which is why there is no "planned"
// state: nothing waits on a human writing a line first.
type GroupEntry struct {
	// CommandGroup is the command path the group's methods live under, or
	// should live under once scaffolded ("virtual-machines backups"). The SDK
	// splits by resource where the CLI groups by product: VirtualMachineBackups
	// and VirtualMachineRestores both belong under `virtual-machines`. When
	// unset, the group is inferred from the mapped commands, or from the SDK
	// group name for a group with none.
	CommandGroup string `yaml:"command_group,omitempty"`

	// Commands maps an SDK method to the command path(s) that call it. Every
	// path is verified against the cobra tree.
	Commands map[string]Paths `yaml:"commands,omitempty"`

	// Excluded maps an SDK method to the reason it has no command.
	Excluded map[string]Exclusion `yaml:"excluded,omitempty"`

	// ExcludedGroup leaves the whole group out with one reason, for a product
	// that is not ready for the CLI (a preview behind a feature flag, a niche
	// flow). Every method not mapped under Commands counts as excluded; the
	// group is not scaffolded and does not show as pending.
	ExcludedGroup *Exclusion `yaml:"excluded_group,omitempty"`

	// Suggested overrides the command the report proposes for a pending
	// method, for cases where the UX was decided ahead of the code and the
	// automatic naming would get it wrong: a BGP session is "a server
	// announcing the elastic IP", so it is `elastic-ips attach --server`, not
	// a `bgp-sessions` subgroup. The method stays pending; only its name in
	// the report and the scaffold prompt changes. Several methods may share a
	// suggestion (two routes, one command).
	Suggested map[string]string `yaml:"suggested,omitempty"`

	// Extends maps a pending method to the existing command that is to call
	// it: the UX grows that command (a flag, a column) instead of adding one.
	// Listing an elastic IP's BGP sessions belongs in `elastic-ips get`, which
	// does not call ListElasticIPBgpSessions yet. Unlike Suggested, the path
	// must be a registered leaf, so a rename is caught; unlike Commands, the
	// method stays pending until the command calls it, and the entry then
	// moves to Commands.
	Extends map[string]string `yaml:"extends,omitempty"`

	// Notes is free-form context for whoever scaffolds or reviews this group
	// next.
	Notes string `yaml:"notes,omitempty"`
}

// Manifest is the hand-curated record of intent, keyed by the SDK's dotted group
// path.
type Manifest struct {
	Version   int                   `yaml:"version"`
	SDKModule string                `yaml:"sdk_module"`
	Groups    map[string]GroupEntry `yaml:"groups"`

	// CommandsWithoutSDK lists leaf commands that call no SDK method: local
	// state (login, profile), self-update, or a hand-written HTTP call. They are
	// verified against the cobra tree too, so a removed command cannot linger
	// here.
	CommandsWithoutSDK []string `yaml:"commands_without_sdk,omitempty"`
}

// LoadManifest reads and parses the manifest at path.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("sdkcoverage: reading manifest: %w", err)
	}

	var manifest Manifest
	// KnownFields makes a typo in a manifest key an error rather than a setting
	// that silently does nothing.
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("sdkcoverage: parsing %s: %w", path, err)
	}

	if manifest.Version != SupportedManifestVersion {
		return Manifest{}, fmt.Errorf(
			"sdkcoverage: %s declares version %d, but this build understands version %d",
			path, manifest.Version, SupportedManifestVersion)
	}
	if manifest.SDKModule != SDKModulePath {
		return Manifest{}, fmt.Errorf(
			"sdkcoverage: %s declares sdk_module %q, but coverage is reconciled against %q",
			path, manifest.SDKModule, SDKModulePath)
	}

	if manifest.Groups == nil {
		manifest.Groups = map[string]GroupEntry{}
	}
	return manifest, nil
}

func knownReasons() []Reason {
	return []Reason{ReasonAPIConstraint, ReasonProductDecision, ReasonDeprecated}
}

func validReason(r Reason) bool {
	for _, known := range knownReasons() {
		if r == known {
			return true
		}
	}
	return false
}
