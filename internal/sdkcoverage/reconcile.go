package sdkcoverage

import (
	"fmt"
	"sort"
	"strings"
)

// Violation is a disagreement between the SDK surface, the manifest, and the
// commands the CLI actually ships. Every violation is actionable by a human
// editing the manifest or the CLI.
//
// Violations are limited to contradictions. A method or group with no entry is
// deliberately not one: that is the normal trigger for scaffolding, so an SDK
// bump does not go red over new capability.
type Violation struct {
	// Group is the SDK group the violation concerns, or "" when it concerns a
	// command with no owning group.
	Group   string
	Message string
}

func (v Violation) String() string {
	if v.Group == "" {
		return v.Message
	}
	return v.Group + ": " + v.Message
}

// Method is one row of the per-method ledger.
type Method struct {
	Group    string
	Name     string
	Commands Paths      // non-empty when covered
	Excluded *Exclusion // non-nil when deliberately left out
}

// Covered reports whether at least one command calls the method.
func (m Method) Covered() bool { return len(m.Commands) > 0 }

// Pending reports whether the method is neither covered nor excluded.
func (m Method) Pending() bool { return !m.Covered() && m.Excluded == nil }

// GroupReport is one SDK group with its methods classified.
type GroupReport struct {
	Group
	Notes   string
	Methods []Method

	// commandGroup is the manifest's command_group, when set.
	commandGroup string
	// newCommandGroup is true when no registered leaf lives under CommandGroup.
	newCommandGroup bool
	// suggested is the manifest's per-method override of the proposed command.
	suggested map[string]string
	// extends is the manifest's per-method existing command to grow.
	extends map[string]string
}

// CommandGroup is the command path the group's methods live (or should live)
// under: the manifest's command_group, else the shortest parent of the mapped
// commands (ties broken alphabetically), else the kebab-cased SDK group name.
// Taking the shortest parent rather than the first one keeps the answer
// independent of SDK method order: Firewalls maps both `firewalls list` and
// `firewalls assignments create`, and its group is `firewalls`. When every
// mapped command sits in a subgroup (`lks node-pools ...`), no inference can
// tell, and the manifest should set command_group.
func (g GroupReport) CommandGroup() string {
	if g.commandGroup != "" {
		return g.commandGroup
	}
	best := ""
	for _, m := range g.Methods {
		for _, p := range m.Commands {
			i := strings.LastIndex(p, " ")
			if i <= 0 {
				continue
			}
			parent, depth := p[:i], strings.Count(p[:i], " ")
			if best == "" || depth < strings.Count(best, " ") || (depth == strings.Count(best, " ") && parent < best) {
				best = parent
			}
		}
	}
	if best != "" {
		return best
	}
	return kebab(lastSegment(g.Name))
}

// NewCommandGroup reports whether the top-level command of the group's
// command group does not exist in the CLI yet: scaffolding it means a new
// root command (`lks`), not a subgroup or subcommand on an existing one
// (`virtual-machines backups`). Set by Reconcile against the registered
// leaves.
func (g GroupReport) NewCommandGroup() bool { return g.newCommandGroup }

// topLevel is the first segment of a command path.
func topLevel(path string) string {
	if i := strings.Index(path, " "); i > 0 {
		return path[:i]
	}
	return path
}

// Pending returns the group's pending methods.
func (g GroupReport) Pending() []Method {
	var out []Method
	for _, m := range g.Methods {
		if m.Pending() {
			out = append(out, m)
		}
	}
	return out
}

// Suggestion is one command the pending work would add. It usually backs a
// single SDK method, but the SDK exposes some operations through two routes
// (POST /virtual_machine_backups and POST /virtual_machines/{id}/backups) and
// those become one command with a flag, so a suggestion can carry several
// methods.
type Suggestion struct {
	Command string
	Methods []string
	// Extends is true when Command already exists and grows to call the
	// methods (the manifest's `extends`), rather than being added.
	Extends bool
}

// PendingSuggestions collapses the group's pending methods into commands.
// Two methods merge when they share the HTTP verb and their paths end the
// same way (the same resource, one nested under a parent): the command takes
// the shorter method's name and the parent becomes a flag. A name the
// manifest gives (suggested or extends) wins over the automatic one: named
// methods seed their command first, so a sibling route joins them rather than
// folding them under an automatic name, and a named method joins another
// command only by sharing its name.
func (g GroupReport) PendingSuggestions() []Suggestion {
	pending := g.Pending()
	merged := make(map[string]bool, len(pending))
	names := g.suggestions()
	extended := make(map[string]bool, len(g.extends))
	for _, cmd := range g.extends {
		extended[strings.TrimSpace(cmd)] = true
	}
	named := make(map[string]bool, len(g.suggested)+len(g.extends))
	for _, overrides := range []map[string]string{g.suggested, g.extends} {
		for method := range overrides {
			named[method] = true
		}
	}
	seeds := make([]Method, 0, len(pending))
	for _, first := range []bool{true, false} {
		for _, m := range pending {
			if named[m.Name] == first {
				seeds = append(seeds, m)
			}
		}
	}
	var out []Suggestion

	for _, m := range seeds {
		if merged[m.Name] {
			continue
		}
		s := Suggestion{Command: names[m.Name], Methods: []string{m.Name}, Extends: extended[names[m.Name]]}
		merged[m.Name] = true
		// Absorb every method that is one command with any method already in
		// s: the same name, or a second route to one of them. Membership is
		// transitive (List's nested route joins the command List was folded
		// into), so keep sweeping until nothing new joins.
		for changed := true; changed; {
			changed = false
			for _, other := range pending {
				if merged[other.Name] {
					continue
				}
				join := names[other.Name] == s.Command
				for _, member := range s.Methods {
					if join || named[other.Name] {
						break
					}
					join = sameOperation(g.Routes[member], g.Routes[other.Name])
				}
				if join {
					s.Methods = append(s.Methods, other.Name)
					merged[other.Name] = true
					changed = true
				}
			}
		}
		out = append(out, s)
	}
	// Subgroups read together, verbs in a stable order inside each.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}

// sameOperation reports whether two routes are one operation reachable
// through a top-level and a nested path. They must share the verb, address
// the same kind of target (both a collection, or both one item), and the
// top-level spelling must name the nested resource with its parent as a
// prefix: "/virtual_machine_restores" and
// "/virtual_machines/{}/backups/{}/restores" qualify (restores under a
// backup, spelled virtual_machine_restores at the top). "/servers" and
// "/servers/{}" do not (a list and a get), and neither do
// "/storage/access_keys" and "/storage/buckets/{}/access_keys": the same
// final name under a different parent is a different listing, and the
// top-level route does not spell the parent.
func sameOperation(a, b Route) bool {
	if a.Verb == "" || b.Verb == "" || a.Verb != b.Verb {
		return false
	}
	pa, pb := a.Normalized(), b.Normalized()
	if pa == pb {
		return true
	}
	ra, ia := resourceAndItem(pa)
	rb, ib := resourceAndItem(pb)
	if ia != ib || ra == "" || rb == "" {
		return false
	}
	return strings.HasSuffix(ra, "_"+rb) || strings.HasSuffix(rb, "_"+ra)
}

// resourceAndItem splits a normalized path into its final resource name and
// whether it addresses one item (ends in "/{}") rather than the collection.
func resourceAndItem(p string) (resource string, item bool) {
	if strings.HasSuffix(p, "/{}") {
		item = true
		p = strings.TrimSuffix(p, "/{}")
	}
	return p[strings.LastIndex(p, "/")+1:], item
}

// Covered returns the group's covered methods.
func (g GroupReport) Covered() []Method {
	var out []Method
	for _, m := range g.Methods {
		if m.Covered() {
			out = append(out, m)
		}
	}
	return out
}

// NewGroup reports whether no method of the group has a command yet: the
// group needs a new command group, not a subcommand on an existing one.
func (g GroupReport) NewGroup() bool { return len(g.Covered()) == 0 }

// Report is the full reconciliation result.
type Report struct {
	Violations []Violation
	Groups     []GroupReport // every SDK group, sorted by name

	// leaves are the registered commands the report was reconciled against;
	// the renderers use them to tell a new top-level command from a new
	// subcommand.
	leaves []string
}

// OK reports whether the SDK, the manifest, and the CLI all agree.
func (r Report) OK() bool { return len(r.Violations) == 0 }

// Counts summarises the ledger. The JSON names are part of the contract the
// sdk-watch workflow reads; keep them stable.
type Counts struct {
	Groups int `json:"groups"`
	// NewGroups is the number of SDK groups with no command at all.
	NewGroups int `json:"new_groups"`
	// NewCommandGroups is the number of distinct top-level commands the
	// pending work would add to the CLI.
	NewCommandGroups int `json:"new_command_groups"`
	Methods          int `json:"methods"`
	Covered          int `json:"covered"`
	Excluded         int `json:"excluded"`
	Pending          int `json:"pending"`
	Violations       int `json:"violations"`
}

func (r Report) Counts() Counts {
	c := Counts{Groups: len(r.Groups), Violations: len(r.Violations)}
	newRoots := map[string]bool{}
	cmds, newRoot := r.PendingByRoot()
	for _, p := range cmds {
		if newRoot[p.Root()] {
			newRoots[p.Root()] = true
		}
	}
	for _, g := range r.Groups {
		if g.NewGroup() {
			c.NewGroups++
		}
		for _, m := range g.Methods {
			c.Methods++
			switch {
			case m.Covered():
				c.Covered++
			case m.Excluded != nil:
				c.Excluded++
			default:
				c.Pending++
			}
		}
	}
	c.NewCommandGroups = len(newRoots)
	return c
}

// PendingGroups returns the groups that still have pending methods, ordered by
// command group so SDK groups that share one (VirtualMachines,
// VirtualMachineBackups, VirtualMachineRestores) read together, with the
// groups whose command group does not exist yet first.
func (r Report) PendingGroups() []GroupReport {
	var out []GroupReport
	for _, g := range r.Groups {
		if len(g.Pending()) > 0 {
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NewCommandGroup() != out[j].NewCommandGroup() {
			return out[i].NewCommandGroup()
		}
		if ci, cj := out[i].CommandGroup(), out[j].CommandGroup(); ci != cj {
			return ci < cj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// PendingCommand is one suggested command with the SDK group and methods
// behind it. The report is organised around these rather than around SDK
// groups, because one SDK group can feed several command groups (Plans
// lists the LKS, database and storage catalogs, which belong next to their
// products) and several SDK groups can feed one (backups and restores).
type PendingCommand struct {
	Command string
	Group   string   // SDK group
	Methods []string // SDK methods behind the command
	Notes   string   // the SDK group's manifest notes
	Extends bool     // the command exists and grows; see Suggestion.Extends
}

// Root is the command's top-level command (`lks` for `lks plans list`),
// which is how the report groups pending work: everything that lands under
// one root reads together, whatever SDK groups feed it.
func (p PendingCommand) Root() string { return topLevel(p.Command) }

// PendingByRoot returns every suggested command, sorted by its root command
// and then by command, with the roots that do not exist in the CLI yet
// first. newRoot reports, per root, whether it is new.
func (r Report) PendingByRoot() (cmds []PendingCommand, newRoot map[string]bool) {
	existing := map[string]bool{}
	for _, l := range r.leaves {
		existing[topLevel(l)] = true
	}
	newRoot = map[string]bool{}
	for _, g := range r.Groups {
		for _, s := range g.PendingSuggestions() {
			p := PendingCommand{Command: s.Command, Group: g.Name, Methods: s.Methods, Notes: g.Notes, Extends: s.Extends}
			cmds = append(cmds, p)
			newRoot[p.Root()] = !existing[p.Root()]
		}
	}
	sort.SliceStable(cmds, func(i, j int) bool {
		ri, rj := cmds[i].Root(), cmds[j].Root()
		if newRoot[ri] != newRoot[rj] {
			return newRoot[ri]
		}
		if ri != rj {
			return ri < rj
		}
		return cmds[i].Command < cmds[j].Command
	})
	return cmds, newRoot
}

// Reconcile cross-checks the SDK surface, the manifest, and the leaf commands
// the CLI registers.
//
// leaves is passed in as plain strings so this package never imports the
// command packages: the gate test lives next to them and would otherwise be an
// import cycle.
func Reconcile(surface Surface, manifest Manifest, leaves []string) Report {
	report := Report{leaves: leaves}

	leafSet := make(map[string]bool, len(leaves))
	for _, l := range leaves {
		leafSet[l] = true
	}
	claimed := make(map[string]string, len(leaves)) // leaf -> "Group.Method"

	addViolation := func(group, format string, args ...any) {
		report.Violations = append(report.Violations, Violation{Group: group, Message: fmt.Sprintf(format, args...)})
	}

	for _, name := range surface.Names() {
		group := surface.Groups[name]
		entry := manifest.Groups[name] // zero value is a valid "no entry"
		row := GroupReport{Group: group, Notes: entry.Notes, commandGroup: entry.CommandGroup, suggested: entry.Suggested, extends: entry.Extends}
		known := make(map[string]bool, len(group.Methods))

		if ex := entry.ExcludedGroup; ex != nil {
			if ex.Reason == "" {
				addViolation(name, "excluded_group needs a reason (%s)", reasonList())
			} else if !validReason(ex.Reason) {
				addViolation(name, "excluded_group has unknown reason %q (want %s)", ex.Reason, reasonList())
			}
			if len(entry.Excluded) > 0 {
				addViolation(name, "excluded_group already covers every unmapped method; drop the per-method excluded entries")
			}
		}

		for _, method := range group.Methods {
			known[method] = true
			m := Method{Group: name, Name: method}

			if paths, ok := entry.Commands[method]; ok {
				if len(paths) == 0 {
					addViolation(name, "commands.%s lists no command path", method)
				}
				for _, p := range paths {
					if !leafSet[p] {
						addViolation(name, "commands.%s names %q, which the CLI does not register (`sdkcoverage commands` lists the current paths)", method, p)
						continue
					}
					claimed[p] = name + "." + method
				}
				m.Commands = paths
			}
			if ex, ok := entry.Excluded[method]; ok {
				if m.Covered() {
					addViolation(name, "%s is both mapped under commands and listed under excluded; keep one", method)
				}
				if ex.Reason == "" {
					addViolation(name, "excluded.%s needs a reason (%s)", method, reasonList())
				} else if !validReason(ex.Reason) {
					addViolation(name, "excluded.%s has unknown reason %q (want %s)", method, ex.Reason, reasonList())
				}
				ex := ex
				m.Excluded = &ex
			}
			if !m.Covered() && m.Excluded == nil && entry.ExcludedGroup != nil {
				m.Excluded = entry.ExcludedGroup
			}
			row.Methods = append(row.Methods, m)
		}

		// Entries for methods the SDK no longer exposes: a rename or removal
		// on an SDK bump that would otherwise pass unnoticed.
		for _, method := range sortedKeys(entry.Commands) {
			if !known[method] {
				addViolation(name, "commands.%s: the SDK no longer exposes this method; rename or drop the entry", method)
			}
		}
		for _, method := range sortedKeys(entry.Excluded) {
			if !known[method] {
				addViolation(name, "excluded.%s: the SDK no longer exposes this method; drop the entry", method)
			}
		}
		for _, method := range sortedKeys(entry.Suggested) {
			switch {
			case !known[method]:
				addViolation(name, "suggested.%s: the SDK no longer exposes this method; drop the entry", method)
			case strings.TrimSpace(entry.Suggested[method]) == "":
				addViolation(name, "suggested.%s is empty; give it a command path or drop it", method)
			case len(entry.Commands[method]) > 0:
				addViolation(name, "suggested.%s: the method is already mapped under commands, so the suggestion is moot; drop it", method)
			case leafSet[strings.TrimSpace(entry.Suggested[method])]:
				// A suggestion is a name for a command that does not exist
				// yet. Naming one that does means the method is covered by
				// it (commands), is to be added to it (extends), or is not
				// what that command does (pick another name).
				addViolation(name, "suggested.%s names %q, which the CLI already registers; map the method under commands if that command calls it, under extends if it is to call it, or suggest a new name", method, strings.TrimSpace(entry.Suggested[method]))
			}
		}
		for _, method := range sortedKeys(entry.Extends) {
			target := strings.TrimSpace(entry.Extends[method])
			_, suggested := entry.Suggested[method]
			_, excluded := entry.Excluded[method]
			switch {
			case !known[method]:
				addViolation(name, "extends.%s: the SDK no longer exposes this method; drop the entry", method)
			case target == "":
				addViolation(name, "extends.%s is empty; name the command to extend or drop it", method)
			case len(entry.Commands[method]) > 0:
				// The usual way this happens is the extension landing: the
				// command now calls the method and was mapped, so the plan
				// is done.
				addViolation(name, "extends.%s: the method is already mapped under commands, so the extension is done; drop it", method)
			case suggested:
				addViolation(name, "extends.%s: the method also has a suggested command; keep one", method)
			case excluded || entry.ExcludedGroup != nil:
				addViolation(name, "extends.%s: the method is excluded, so there is nothing to extend; keep one", method)
			case !leafSet[target]:
				addViolation(name, "extends.%s names %q, which the CLI does not register; extend an existing command, or use suggested for a new one", method, target)
			}
		}

		// A command group is "new" when its top-level command does not exist.
		// The SDK's VirtualMachineBackups has no command, yet its
		// command_group `virtual-machines backups` nests under an existing
		// root: the scaffold adds a subgroup, not a top-level command.
		row.newCommandGroup = true
		for _, leaf := range leaves {
			if topLevel(leaf) == topLevel(row.CommandGroup()) {
				row.newCommandGroup = false
				break
			}
		}
		report.Groups = append(report.Groups, row)
	}

	for _, name := range sortedKeys(manifest.Groups) {
		if _, ok := surface.Groups[name]; !ok {
			addViolation(name, "listed in %s but the SDK no longer exposes this group; rename or delete the entry (`sdkcoverage groups` lists the current names)", ManifestFile)
		}
	}

	// commands_without_sdk must name real leaves, and must not overlap with a
	// method mapping: a leaf cannot both call an SDK method and call none.
	withoutSDK := make(map[string]bool, len(manifest.CommandsWithoutSDK))
	for _, p := range manifest.CommandsWithoutSDK {
		withoutSDK[p] = true
		if !leafSet[p] {
			addViolation("", "commands_without_sdk names %q, which the CLI does not register", p)
		} else if owner, ok := claimed[p]; ok {
			addViolation("", "commands_without_sdk lists %q, but %s already maps it; keep one", p, owner)
		}
	}

	// Every leaf must be accounted for, otherwise the method it calls still
	// reads as pending and would be scaffolded a second time.
	for _, leaf := range leaves {
		if _, ok := claimed[leaf]; ok || withoutSDK[leaf] {
			continue
		}
		addViolation("", "the CLI registers %q but no SDK method claims it; add it under the method it calls, or under commands_without_sdk", leaf)
	}

	// Two SDK groups placed under one command group must not propose the
	// same command: Teams.Get and UserProfile.Get both read as `teams get`
	// when UserProfile is filed under teams, and the scaffold would build
	// one command over the other. A collision inside one group is fine
	// (it is how two routes fold into one command) and is not reported,
	// and neither is an extension: two groups may grow the same command.
	owners := map[string][]string{}
	for _, g := range report.Groups {
		for _, s := range g.PendingSuggestions() {
			if s.Extends {
				continue
			}
			owners[s.Command] = append(owners[s.Command], g.Name)
		}
	}
	for _, cmd := range sortedKeys(owners) {
		if groups := owners[cmd]; len(groups) > 1 {
			sort.Strings(groups)
			addViolation("", "%s would be suggested for both %s; move one group to another command_group or name its command under suggested", cmd, strings.Join(groups, " and "))
		}
	}

	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Group != report.Violations[j].Group {
			return report.Violations[i].Group < report.Violations[j].Group
		}
		return report.Violations[i].Message < report.Violations[j].Message
	})
	return report
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func reasonList() string {
	names := make([]string, 0, len(knownReasons()))
	for _, r := range knownReasons() {
		names = append(names, string(r))
	}
	return strings.Join(names, ", ")
}
