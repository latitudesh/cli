package sdkcoverage

import (
	"sort"
	"strings"
)

// verbPrefixes maps an SDK method-name prefix to the CLI verb it usually
// becomes. Prefix matching is deliberate: the generated SDK is not consistent
// (SSHKeys.Retrieve, BlockStorage.ListVolumes, PublicNetworks.DestroyPublicNetwork
// all have to land on the right verb).
var verbPrefixes = map[string]string{
	"Create":     "create",
	"Post":       "create",
	"Assign":     "assign",
	"List":       "list",
	"Get":        "get",
	"Retrieve":   "get",
	"Fetch":      "get",
	"Show":       "get",
	"Update":     "update",
	"Patch":      "update",
	"Put":        "update",
	"Modify":     "update",
	"Delete":     "delete",
	"Destroy":    "delete",
	"Remove":     "delete",
	"Lock":       "lock",
	"Unlock":     "unlock",
	"Start":      "start",
	"Exit":       "exit",
	"Run":        "run",
	"Reinstall":  "reinstall",
	"Schedule":   "schedule",
	"Unschedule": "unschedule",
	"Refresh":    "refresh",
	"Mount":      "mount",
	"Map":        "map",
	"Unmap":      "unmap",
	"Attach":     "attach",
	"Detach":     "detach",
	"Exec":       "exec",
	"Sign":       "sign",
}

// orderedVerbPrefixes is verbPrefixes sorted longest-first so the most specific
// prefix wins: "Unschedule" before "Schedule", "Unlock" before "Lock".
var orderedVerbPrefixes = func() []string {
	prefixes := make([]string, 0, len(verbPrefixes))
	for p := range verbPrefixes {
		prefixes = append(prefixes, p)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if len(prefixes[i]) != len(prefixes[j]) {
			return len(prefixes[i]) > len(prefixes[j])
		}
		return prefixes[i] < prefixes[j]
	})
	return prefixes
}()

// SuggestCommand proposes a command path for an SDK method, e.g.
// Lks.CreateLksNodePool under "lks" → "lks create-node-pool". It is a hint
// for the tracking issue and the scaffold prompt, not a decision: the verb
// comes from the method-name prefix, with the words that merely repeat the SDK
// group or the command group stripped from the remainder.
func SuggestCommand(group Group, groupCommand string, method string) string {
	return suggestCommand(group, groupCommand, method, true)
}

// SuggestSubcommand is SuggestCommand for a group that already has commands
// under groupCommand: the command group's noun is settled (BlockStorage is
// `volume`), so the SDK group's own noun is never reinjected into the leaf.
func SuggestSubcommand(group Group, groupCommand string, method string) string {
	return suggestCommand(group, groupCommand, method, false)
}

func suggestCommand(group Group, groupCommand string, method string, keepGroupNoun bool) string {
	groupCommand, verb, rest := splitMethod(group, groupCommand, method, keepGroupNoun)
	if verb == "" {
		return groupCommand + " " + kebab(method)
	}
	if rest == "" {
		return groupCommand + " " + verb
	}
	return groupCommand + " " + verb + "-" + kebab(rest)
}

// splitMethod breaks a method into the command group it lands under, the
// CLI verb and the CamelCase remainder specific to the method. verb is ""
// when no known prefix matches.
func splitMethod(group Group, groupCommand string, method string, keepGroupNoun bool) (string, string, string) {
	if groupCommand == "" {
		groupCommand = kebab(lastSegment(group.Name))
	}

	verb, rest := "", method
	for _, p := range orderedVerbPrefixes {
		if strings.HasPrefix(method, p) {
			verb, rest = verbPrefixes[p], method[len(p):]
			break
		}
	}
	if verb == "" {
		return groupCommand, "", method
	}

	// Drop the words that just repeat the SDK group ("Lks", "VirtualMachine",
	// "ElasticIP") or the command group ("virtual-machines backups" already
	// says "backup"), so the leaf is the verb plus what is specific to the
	// method. A "Get" whose remainder is the plural noun (GetBaselines,
	// GetPublicNetworks) is a list, not a lookup.
	rest, plural := stripKnownWords(rest, group, groupCommand)
	if verb == "get" && plural && rest == "" {
		verb = "list"
	}

	// When the SDK group's own noun is absent from the command group, a bare
	// method (Create, List) must carry it or two groups sharing a command
	// group collide: VirtualMachineRestores and VirtualMachineBackups both map
	// under `virtual-machines backups`, so Restores.Create is `backups
	// create-restore`, not a second `backups create`. A method that already
	// spelled the noun (ObjectStorage.GetStorageAccessKeys) does not get it
	// back: the strip above removed it as group noise, on purpose.
	if keepGroupNoun && !strings.Contains(method, lastSegment(group.Name)) {
		if noun := groupNounOutside(group, groupCommand); noun != "" && !strings.Contains(method, noun) {
			rest = noun + rest
		}
	}
	return groupCommand, verb, rest
}

// crudVerbs are the verbs that, applied to the same noun, mean the noun is a
// resource of its own and deserves a subgroup rather than a hyphenated leaf.
var crudVerbs = map[string]bool{"create": true, "list": true, "get": true, "update": true, "delete": true}

// collectionAdjectives qualify which members of a collection a list returns
// (ListLksAvailableVersions, ListLksAllocatableServers). The subgroup is the
// collection itself, so they drop out of its name.
var collectionAdjectives = map[string]bool{"available": true, "allocatable": true, "all": true}

// SuggestCommands proposes one command per method for a whole group at once,
// which lets it spot resources the SDK flattened into method names. A noun
// that is a collection becomes a subgroup named after it with plain verbs
// under it, as `kubernetes clusters create` and `kubernetes versions list`
// already read:
//
//   - a noun seen with several CRUD verbs (CreateLksNodePool, GetLksNodePool,
//     ListLksNodePools, ...) is a resource: `lks node-pools create`;
//   - a noun seen with list alone (ListLksSites, ListLksAvailableVersions) is
//     a reference collection: `lks sites list`, `lks versions list`;
//   - a noun seen only with get and update (Servers.GetDeployConfig /
//     UpdateDeployConfig) is a singleton setting of its parent, not a
//     collection, and stays a hyphenated leaf (`servers get-deploy-config`).
//
// A method with a resource noun and an extra qualifier
// (GetLksClusterKubeconfig) lands in the resource's subgroup with the
// qualifier on the leaf (`lks clusters get-kubeconfig`).
func SuggestCommands(group Group, groupCommand string, methods []string, keepGroupNoun bool) map[string]string {
	type split struct{ groupCommand, verb, rest string }
	parts := make(map[string]split, len(methods))
	rests := make([]string, 0, len(methods))
	verbsByNoun := map[string]map[string]bool{}
	for _, m := range methods {
		gc, verb, rest := splitMethod(group, groupCommand, m, keepGroupNoun)
		if verb == "list" {
			rest = stripCollectionAdjectives(rest)
		}
		parts[m] = split{gc, verb, rest}
		if verb == "" || rest == "" {
			continue
		}
		rests = append(rests, rest)
		noun := strings.ToLower(singular(rest))
		if verbsByNoun[noun] == nil {
			verbsByNoun[noun] = map[string]bool{}
		}
		verbsByNoun[noun][verb] = true
	}

	// Evidence for a noun's plural: every method remainder, plus the SDK
	// group's own name (VirtualMachineRestores spells Restores in the
	// plural even when its methods are the bare Create/List/Get).
	evidence := append(append([]string(nil), rests...), lastSegment(group.Name))

	resources := map[string]bool{}
	for noun, verbs := range verbsByNoun {
		n := 0
		for v := range verbs {
			if crudVerbs[v] {
				n++
			}
		}
		if verbs["list"] || (n >= 2 && verbs["create"]) {
			resources[noun] = true
		}
	}

	out := make(map[string]string, len(methods))
	for _, m := range methods {
		p := parts[m]
		if p.verb == "" {
			out[m] = p.groupCommand + " " + kebab(m)
			continue
		}
		if p.rest == "" {
			out[m] = p.groupCommand + " " + p.verb
			continue
		}
		// The remainder may be exactly a resource noun (NodePool / NodePools)
		// or start with one followed by a qualifier (ClusterKubeconfig).
		if noun, qualifier, pluralNoun, ok := leadingResource(p.rest, resources); ok {
			// The subgroup is the collection, spelled in the plural unless the
			// SDK itself only ever lists it in the singular (ListUsage,
			// ListStorage): those are mass nouns, and "usages" is not a word.
			sub := p.groupCommand + " " + kebab(collectionName(noun, evidence))
			// A list-only collection that the route nests under another
			// resource of the group belongs to that resource:
			// /lks/clusters/{}/nodepools/{}/nodes lists the nodes of one node
			// pool, so it is `lks node-pools nodes list`, not a top-level
			// `lks nodes list`. A resource with its own CRUD stays top-level
			// even when its route is scoped by a parent (node pools live
			// under a cluster the way VMs live under a project): the parent
			// becomes a flag, as `kubernetes clusters create --project` does.
			if listOnly(verbsByNoun[strings.ToLower(noun)]) {
				if parent := routeParent(group.Routes[m], resources, rests); parent != "" && !strings.EqualFold(parent, noun) {
					sub = p.groupCommand + " " + kebab(collectionName(parent, evidence)) + " " + kebab(collectionName(noun, evidence))
				}
			}
			verb := p.verb
			// GetStorageBucketLifecycleRules next to GetStorageBucketLifecycleRule:
			// the plural get is the list.
			if verb == "get" && pluralNoun && qualifier == "" {
				verb = "list"
			}
			if qualifier == "" {
				out[m] = sub + " " + verb
			} else {
				out[m] = sub + " " + verb + "-" + kebab(qualifier)
			}
			continue
		}
		out[m] = p.groupCommand + " " + p.verb + "-" + kebab(p.rest)
	}
	return out
}

// listOnly reports whether a noun was seen with the list verb and nothing
// else: a collection that is read but never created or deleted here.
func listOnly(verbs map[string]bool) bool {
	return len(verbs) == 1 && verbs["list"]
}

// routeParent returns the resource noun (in the group's own CamelCase
// spelling) that the route's final segment is nested under, when that parent
// is itself a resource of the group. For
// /lks/clusters/{}/nodepools/{}/nodes the parent segment is "nodepools",
// which matches the NodePool resource; for /lks/clusters/{}/kubeconfig the
// parent is "clusters". A route with no nesting, or whose parent is not a
// resource of this group (the project in /projects/{}/user_data), yields "".
func routeParent(route Route, resources map[string]bool, rests []string) string {
	if route.Path == "" {
		return ""
	}
	segs := strings.Split(strings.Trim(route.Normalized(), "/"), "/")
	// Walk back from the final segment past its own id, then past the
	// parent's id, to the parent segment.
	i := len(segs) - 1
	if segs[i] == "{}" {
		i--
	}
	if i < 1 {
		return ""
	}
	i-- // the segment before the final resource
	if i >= 0 && segs[i] == "{}" {
		i--
	}
	if i < 0 {
		return ""
	}
	parent := strings.ReplaceAll(segs[i], "_", "")
	for noun := range resources {
		if strings.EqualFold(singular(parent), strings.ReplaceAll(noun, " ", "")) ||
			strings.EqualFold(parent, plural(noun)) || strings.EqualFold(parent, noun) {
			// Return the CamelCase spelling as the method remainders use it.
			for _, rest := range rests {
				if strings.EqualFold(singular(rest), noun) || strings.EqualFold(rest, noun) {
					return singular(rest)
				}
			}
			return noun
		}
	}
	return ""
}

// collectionName spells a resource noun as a subgroup. The plural is the
// norm (`clusters`, `node-pools`); when no method of the group ever spells
// the noun in the plural, the SDK is treating it as a mass noun (Usage,
// Storage) and the singular is kept. evidence is every method remainder the
// nouns were taken from, plus the group's own name.
func collectionName(noun string, evidence []string) string {
	for _, rest := range evidence {
		// The noun may be the whole remainder (NodePools) or lead it
		// (LifecycleRules + qualifier); compare the leading span of the
		// same word count, so compound nouns are matched whole.
		words := splitWords(rest)
		n := len(splitWords(noun))
		if n == 0 || n > len(words) {
			continue
		}
		// The noun may lead the remainder (NodePools, LifecycleRulesFoo) or
		// end the group name (VirtualMachineRestores); check both spans.
		for _, span := range []string{strings.Join(words[:n], ""), strings.Join(words[len(words)-n:], "")} {
			if strings.EqualFold(singular(span), noun) && !strings.EqualFold(span, noun) {
				return plural(noun) // seen in the plural somewhere: a count noun
			}
		}
	}
	return noun
}

// stripCollectionAdjectives drops leading adjectives that only qualify which
// members a list returns: AvailableVersions → Versions.
func stripCollectionAdjectives(rest string) string {
	words := splitWords(rest)
	for len(words) > 1 && collectionAdjectives[strings.ToLower(words[0])] {
		words = words[1:]
	}
	return strings.Join(words, "")
}

// leadingResource finds the longest resource noun that rest starts with, and
// returns it together with what follows and whether rest spelled it in the
// plural. Nouns are compared singularized and lower-cased; the returned noun
// keeps the CamelCase spelling from rest.
func leadingResource(rest string, resources map[string]bool) (noun, qualifier string, pluralNoun, ok bool) {
	words := splitWords(rest)
	for n := len(words); n >= 1; n-- {
		candidate := strings.Join(words[:n], "")
		if resources[strings.ToLower(singular(candidate))] {
			return singular(candidate), strings.Join(words[n:], ""), singular(candidate) != candidate, true
		}
	}
	return "", "", false, false
}

// plural is the inverse of singular for the nouns the SDK uses.
func plural(w string) string {
	lw := strings.ToLower(w)
	switch {
	case strings.HasSuffix(lw, "y") && !strings.HasSuffix(lw, "ey"):
		return w[:len(w)-1] + "ies"
	case strings.HasSuffix(lw, "s") || strings.HasSuffix(lw, "x") || strings.HasSuffix(lw, "ch") || strings.HasSuffix(lw, "sh"):
		return w + "es"
	}
	return w + "s"
}

// stripKnownWords removes the words of rest that name the SDK group or the
// command group, and reports whether the last removed word was plural.
//
// ManagedDatabases.CreateManagedDatabase under `databases` must strip
// "ManagedDatabase" even though "managed" is not in the command group, while
// Lks.CreateLksNodePool must keep "NodePool".
func stripKnownWords(rest string, group Group, groupCommand string) (string, bool) {
	known := map[string]bool{}
	for _, w := range splitWords(lastSegment(group.Name)) {
		known[strings.ToLower(singular(w))] = true
	}
	for _, w := range commandWords(groupCommand) {
		known[strings.ToLower(singular(w))] = true
	}
	var kept []string
	plural := false
	for _, w := range splitWords(rest) {
		if known[strings.ToLower(singular(w))] {
			plural = singular(w) != w
			continue
		}
		kept = append(kept, w)
	}
	return strings.Join(kept, ""), plural
}

// groupNounOutside returns the last word of the SDK group name, singular,
// when no word of the command group matches it; "" otherwise. The last word
// is the noun the group is about (VirtualMachineRestores → Restore).
func groupNounOutside(group Group, groupCommand string) string {
	words := splitWords(lastSegment(group.Name))
	if len(words) == 0 {
		return ""
	}
	noun := singular(words[len(words)-1])
	for _, w := range commandWords(groupCommand) {
		if strings.EqualFold(singular(w), noun) {
			return ""
		}
	}
	// A group named after an acronym or a suffix like "Preview" carries no
	// useful noun; the words that make it up are already stripped above.
	if strings.EqualFold(noun, "preview") || len(words) == 1 && strings.ToUpper(words[0]) == words[0] {
		return ""
	}
	return noun
}

func commandWords(groupCommand string) []string {
	return strings.FieldsFunc(groupCommand, func(r rune) bool { return r == ' ' || r == '-' || r == '_' })
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}
	return path
}

// kebab converts a CamelCase identifier to kebab-case, keeping acronyms
// together: "ElasticIPBgpSessions" → "elastic-ip-bgp-sessions".
func kebab(s string) string {
	words := splitWords(s)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "-")
}

func splitWords(s string) []string {
	var words []string
	start := 0
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		boundary := (isLower(prev) && isUpper(cur)) ||
			(isUpper(prev) && isUpper(cur) && isLower(next))
		if boundary {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	if start < len(runes) {
		words = append(words, string(runes[start:]))
	}
	return words
}

// singular reduces a plural word to its singular form. It covers the patterns
// the SDK's group and method names actually use (Databases, Sandboxes,
// Policies, Metrics, Status) and is only used to compare words, never to
// render them.
func singular(w string) string {
	lw := strings.ToLower(w)
	switch {
	case strings.HasSuffix(lw, "ies"):
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(lw, "xes") || strings.HasSuffix(lw, "ches") || strings.HasSuffix(lw, "shes"):
		return w[:len(w)-2]
	case strings.HasSuffix(lw, "ss") || strings.HasSuffix(lw, "us") || strings.HasSuffix(lw, "is"):
		return w
	case strings.HasSuffix(lw, "s"):
		return w[:len(w)-1]
	}
	return w
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
