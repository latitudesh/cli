package sdkcoverage

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// LeafCommands returns the space-separated path of every command under root
// that does something when run ("servers list", "firewalls assignments
// create"), sorted. Hidden and deprecated commands are skipped, as are cobra's
// own help and completion commands: the manifest describes what users can
// run, not plumbing.
//
// A parent is reported through its children, and also on its own when it is
// runnable: `s3 configure` creates and saves an access key and has an
// `export` child. A parent with no Run only prints help and is not reported.
func LeafCommands(root *cobra.Command) []string {
	var leaves []string
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Deprecated != "" || !sub.IsAvailableCommand() {
				continue
			}
			name := sub.Name()
			if c == root && (name == "help" || name == "completion") {
				continue
			}
			subPath := append(append([]string(nil), path...), name)
			if sub.HasAvailableSubCommands() {
				if sub.Runnable() {
					leaves = append(leaves, strings.Join(subPath, " "))
				}
				walk(sub, subPath)
				continue
			}
			leaves = append(leaves, strings.Join(subPath, " "))
		}
	}
	walk(root, nil)
	sort.Strings(leaves)
	return leaves
}
