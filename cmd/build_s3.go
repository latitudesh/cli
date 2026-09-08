package cmd

import (
	s3 "github.com/latitudesh/lsh/cmd/s3"
)

// s3Cmd is the object storage group (`lsh s3`, aliases `buckets` and the
// legacy `storage-objects`). It replaces cmd/storage_objects.
var s3Cmd = s3.NewGroupCmd()

func init() {
	s3Cmd.AddCommand(s3.NewLsCmd())
	s3Cmd.AddCommand(s3.NewMbCmd())
	s3Cmd.AddCommand(s3.NewRbCmd())
	s3Cmd.AddCommand(s3.NewStatCmd())
	s3Cmd.AddCommand(s3.NewCpCmd())
	s3Cmd.AddCommand(s3.NewMvCmd())
	s3Cmd.AddCommand(s3.NewRmCmd())
	s3Cmd.AddCommand(s3.NewSyncCmd())
	s3Cmd.AddCommand(s3.NewPresignCmd())
	s3Cmd.AddCommand(s3.NewConfigureCmd())
	s3Cmd.AddCommand(s3.NewMetricsCmd())
	s3Cmd.AddCommand(s3.NewUsageCmd())
	s3Cmd.AddCommand(s3.NewAccessKeysCmd())
	s3Cmd.AddCommand(s3.NewLifecycleCmd())
	// Install the shared error/exit-code contract on the whole tree now that
	// every subcommand (and its late-assigned RunE/PreRunE) is in place.
	s3.Finalize(s3Cmd)

	rootCmd.AddCommand(s3Cmd)
}
