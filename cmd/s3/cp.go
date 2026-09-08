package s3

import (
	"github.com/spf13/cobra"
)

// NewCpCmd builds `lsh s3 cp <source> <destination>`.
func NewCpCmd() *cobra.Command {
	var f transferFlags
	cmd := newCmd(&cobra.Command{
		Use:     "cp <source> <destination>",
		Aliases: []string{"copy", "upload", "download"},
		Short:   "Copy files and objects",
		Long: `Copy a local file to a bucket, an object to the local disk, or an object to
another object (server-side, same endpoint; when the two buckets resolve
different access keys the object is streamed through this machine instead,
since one signed copy request cannot read with one key and write with
another). The destination is overwritten
by default (--no-overwrite to skip existing ones),
--recursive copies whole directories or prefixes, and --exclude/--include
filter relative paths (rules apply in order; the last match wins).

` + destinationRulesHelp + `

Uploads larger than 16 MiB use multipart; an interrupted upload is aborted.
To clean up leftovers from killed processes create a lifecycle rule:
'lsh s3 lifecycle create s3://<bucket> --abort-incomplete-multipart-days 1'.

Streams from stdin ('-') are always multipart with 16 MiB parts, enough for
160 GiB. For larger streams pass --expected-size <bytes> so the parts grow to
fit S3's 10000-part limit; the value is only a hint that also drives the
progress meter, and the upload succeeds whatever the real length turns out to be.

The Content-Type is taken from --content-type, then the file extension, then
the first bytes of the file (--no-guess-mime-type stores binary/octet-stream).
Output lines (upload:/download:/copy:) go to stdout; progress, hints and
errors to stderr. --dry-run (or --dryrun) prints the plan without writing.`,
		Example: `  lsh s3 cp ./dump.sql s3://backups/2026/09/
  lsh s3 cp s3://backups/2026/09/dump.sql ./restore/
  lsh s3 cp ./site s3://www --recursive --exclude "*" --include "*.html" --content-type text/html
  lsh s3 cp s3://backups/2026/ ./backups/ --recursive --dryrun
  pg_dump mydb | lsh s3 cp - s3://backups/mydb.sql
  tar cz /data | lsh s3 cp - s3://backups/data.tgz --expected-size 500000000000   # approximate; sizes parts for streams over 160 GiB
  lsh s3 cp s3://backups/report.pdf - > report.pdf
  lsh s3 cp s3://backups/a.txt s3://archive/2026/a.txt --metadata owner=ops,team=infra`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopyCommand(cmd, args, &f, false)
		},
	})
	addTransferFlags(cmd, &f, true)
	return cmd
}
