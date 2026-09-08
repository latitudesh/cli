package s3

import (
	"github.com/spf13/cobra"
)

// NewMvCmd builds `lsh s3 mv <source> <destination>`.
func NewMvCmd() *cobra.Command {
	var f transferFlags
	cmd := newCmd(&cobra.Command{
		Use:     "mv <source> <destination>",
		Aliases: []string{"move"},
		Short:   "Move files and objects",
		Long: `Move a local file into a bucket, an object to the local disk, or an object to
another key (server-side, same endpoint; buckets that resolve different access
keys are streamed through this machine). Each source is deleted only after
its copy succeeded; a failed copy leaves the source untouched. Both operands
local, or source and destination naming the same object, is refused.

` + destinationRulesHelp + `

All cp flags apply (--recursive, --exclude/--include, --content-type,
--metadata, --no-overwrite, --quiet, ...). Output lines use the "move:"
prefix. --dry-run (or --dryrun) prints the plan without copying or deleting.`,
		Example: `  lsh s3 mv ./dump.sql s3://backups/2026/09/
  lsh s3 mv s3://backups/tmp/report.pdf ./reports/
  lsh s3 mv s3://backups/tmp/ s3://backups/archive/2026/ --recursive
  lsh s3 mv ./logs s3://logs/host-1/ --recursive --exclude "*" --include "*.log" --dryrun`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopyCommand(cmd, args, &f, true)
		},
	})
	addTransferFlags(cmd, &f, true)
	return cmd
}
