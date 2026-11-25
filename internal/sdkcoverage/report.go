package sdkcoverage

import (
	"encoding/json"
	"fmt"
	"strings"
)

// suggest proposes a command for a pending method. A group that already has
// commands keeps its settled noun (SuggestSubcommand); a group with none may
// need its own noun to avoid colliding with a sibling that shares the command
// group (SuggestCommand).
func (g GroupReport) suggest(method string) string {
	return g.suggestions()[method]
}

// suggestions names every pending method of the group in one pass, so
// resources the SDK flattened into method names become subgroups. Every
// method of the group is evidence, covered or not: which nouns are resources
// is a property of the SDK, so mapping the CRUD of LKS node pools must not
// move the pending node listing out of `lks node-pools`. A `suggested` or
// `extends` entry in the manifest wins over the automatic name (Reconcile
// rejects a method carrying both).
func (g GroupReport) suggestions() map[string]string {
	names := make([]string, 0, len(g.Methods))
	for _, m := range g.Methods {
		names = append(names, m.Name)
	}
	all := SuggestCommands(g.Group, g.CommandGroup(), names, g.NewGroup())
	out := make(map[string]string, len(all))
	for _, m := range g.Pending() {
		out[m.Name] = all[m.Name]
	}
	for _, overrides := range []map[string]string{g.suggested, g.extends} {
		for method, cmd := range overrides {
			if _, ok := out[method]; ok {
				out[method] = strings.TrimSpace(cmd)
			}
		}
	}
	return out
}

// Markdown renders the report for a tracking issue or a roadmap document.
func (r Report) Markdown(sdkVersion string) string {
	var b strings.Builder
	c := r.Counts()

	b.WriteString("# lsh coverage of `latitudesh-go-sdk`\n\n")
	if sdkVersion != "" {
		fmt.Fprintf(&b, "SDK version: `%s`\n\n", sdkVersion)
	}
	fmt.Fprintf(&b, "%d of %d SDK methods have a command (%d excluded, %d pending). The pending work spans %d new top-level command groups.\n",
		c.Covered, c.Methods, c.Excluded, c.Pending, c.NewCommandGroups)

	if len(r.Violations) > 0 {
		fmt.Fprintf(&b, "\n## Violations (%d)\n\n", len(r.Violations))
		for _, v := range r.Violations {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}

	cmds, newRoot := r.PendingByRoot()
	if len(cmds) > 0 {
		b.WriteString("\n## Pending\n\n")
		b.WriteString("Commands to add, grouped by their top-level command. A root marked *new* does not exist in the CLI yet; a command marked *extend* exists and grows to call the methods (a flag, a column); a command listing several SDK methods is one operation reachable through more than one route.\n")
		last := ""
		for _, p := range cmds {
			if root := p.Root(); root != last {
				label := "`" + root + "`"
				if newRoot[root] {
					label += " *(new)*"
				}
				fmt.Fprintf(&b, "\n### %s\n\n", label)
				b.WriteString("| Command | SDK methods |\n|---|---|\n")
				last = root
			}
			label := "`" + p.Command + "`"
			if p.Extends {
				label += " *(extend)*"
			}
			fmt.Fprintf(&b, "| %s | %s |\n", label, methodRefs(p.Group, p.Methods))
		}
		// Notes once per SDK group that contributed, after the tables.
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, p := range cmds {
			if strings.TrimSpace(p.Notes) == "" || seen[p.Group] {
				continue
			}
			seen[p.Group] = true
			fmt.Fprintf(&b, "_%s:_ %s\n\n", p.Group, strings.TrimSpace(p.Notes))
		}
	}

	var excluded []string
	for _, g := range r.Groups {
		for _, m := range g.Methods {
			if m.Excluded != nil {
				excluded = append(excluded, fmt.Sprintf("| `%s` | `%s` | %s | %s |", g.Name, m.Name, m.Excluded.Reason, orDash(m.Excluded.Notes)))
			}
		}
	}
	if len(excluded) > 0 {
		b.WriteString("\n## Excluded\n\n| Group | Method | Reason | Notes |\n|---|---|---|---|\n")
		b.WriteString(strings.Join(excluded, "\n") + "\n")
	}

	b.WriteString("\n## Covered\n\n| Group | Methods | Commands |\n|---|---|---|\n")
	for _, g := range r.Groups {
		covered := g.Covered()
		if len(covered) == 0 {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | %d/%d | %s |\n", g.Name, len(covered), len(g.Methods), codeList(commandsOf(covered)))
	}

	return b.String()
}

// Text renders a short summary for terminal use.
func (r Report) Text(sdkVersion string) string {
	var b strings.Builder
	c := r.Counts()

	if sdkVersion != "" {
		fmt.Fprintf(&b, "SDK %s\n", sdkVersion)
	}
	fmt.Fprintf(&b, "%d/%d methods covered, %d excluded, %d pending; %d new top-level command groups\n",
		c.Covered, c.Methods, c.Excluded, c.Pending, c.NewCommandGroups)

	if cmds, newRoot := r.PendingByRoot(); len(cmds) > 0 {
		b.WriteString("\nPending:\n")
		last := ""
		for _, p := range cmds {
			if root := p.Root(); root != last {
				label := root
				if newRoot[root] {
					label += " (new command group)"
				}
				fmt.Fprintf(&b, "  %s\n", label)
				last = root
			}
			leaf := leafOf(p.Command, p.Root())
			if p.Extends {
				leaf += " (extend)"
			}
			fmt.Fprintf(&b, "    %-44s %s\n", leaf, methodRefs(p.Group, p.Methods))
		}
	}

	if len(r.Violations) > 0 {
		fmt.Fprintf(&b, "\n%d violation(s):\n", len(r.Violations))
		for _, v := range r.Violations {
			fmt.Fprintf(&b, "  %s\n", v)
		}
	}
	return b.String()
}

// jsonReport is the machine-readable document the sdk-watch workflow reads:
// `jq '.counts.pending'` gates whether there is anything to scaffold, and each
// pending entry carries a suggested command so the tracking issue and the
// scaffold job need no naming logic of their own. Keep the field names stable.
type jsonReport struct {
	SDKModule  string          `json:"sdk_module"`
	SDKVersion string          `json:"sdk_version,omitempty"`
	OK         bool            `json:"ok"`
	Counts     Counts          `json:"counts"`
	Violations []jsonViolation `json:"violations"`
	Pending    []jsonPending   `json:"pending"`
}

type jsonViolation struct {
	Group   string `json:"group,omitempty"`
	Message string `json:"message"`
}

type jsonPending struct {
	// CommandGroup is the top-level command the commands live under.
	CommandGroup string `json:"command_group"`
	// NewCommandGroup is true when that top-level command does not exist in
	// the CLI yet: scaffolding means a new root command, not a subcommand.
	NewCommandGroup bool             `json:"new_command_group"`
	Commands        []jsonSuggestion `json:"commands"`
	// Notes carries the manifest notes of every SDK group feeding this
	// command group, keyed by SDK group.
	Notes map[string]string `json:"notes,omitempty"`
}

// jsonSuggestion is one command to scaffold and the SDK method(s) behind it.
// Several methods means one operation reachable through more than one route;
// the scaffold exposes it once, with a flag for the parent.
type jsonSuggestion struct {
	Command string   `json:"command"`
	Group   string   `json:"group"`
	Methods []string `json:"methods"`
	// Extends is true when Command already exists: the scaffold grows it (a
	// flag, a column) to call the methods instead of creating a command.
	Extends bool `json:"extends"`
}

// JSON renders the report as an indented JSON document.
func (r Report) JSON(sdkVersion string) ([]byte, error) {
	doc := jsonReport{
		SDKModule:  SDKModulePath,
		SDKVersion: sdkVersion,
		OK:         r.OK(),
		Counts:     r.Counts(),
		Violations: make([]jsonViolation, 0, len(r.Violations)),
		Pending:    []jsonPending{},
	}
	for _, v := range r.Violations {
		doc.Violations = append(doc.Violations, jsonViolation{Group: v.Group, Message: v.Message})
	}
	cmds, newRoot := r.PendingByRoot()
	var cur *jsonPending
	for _, p := range cmds {
		if cur == nil || cur.CommandGroup != p.Root() {
			doc.Pending = append(doc.Pending, jsonPending{CommandGroup: p.Root(), NewCommandGroup: newRoot[p.Root()], Commands: []jsonSuggestion{}})
			cur = &doc.Pending[len(doc.Pending)-1]
		}
		cur.Commands = append(cur.Commands, jsonSuggestion{Command: p.Command, Group: p.Group, Methods: p.Methods, Extends: p.Extends})
		if strings.TrimSpace(p.Notes) != "" {
			if cur.Notes == nil {
				cur.Notes = map[string]string{}
			}
			cur.Notes[p.Group] = strings.TrimSpace(p.Notes)
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

// methodRefs renders "Group.Method" references, comma-separated.
func methodRefs(group string, methods []string) string {
	refs := make([]string, len(methods))
	for i, m := range methods {
		refs[i] = group + "." + m
	}
	return strings.Join(refs, ", ")
}

// leafOf strips the command group prefix from a suggested command, so a
// listing under a "virtual-machines backups" heading shows "create", not the
// whole path again.
func leafOf(command, group string) string {
	return strings.TrimPrefix(strings.TrimPrefix(command, group), " ")
}

func commandsOf(methods []Method) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range methods {
		for _, p := range m.Commands {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func codeList(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "`" + item + "`"
	}
	return strings.Join(quoted, ", ")
}
