package s3

import (
	cobra "github.com/spf13/cobra"
)

// NewAccessKeysCmd builds `lsh s3 access-keys` (alias `keys`): the S3
// credentials of a project, managed through the Latitude API and optionally
// saved in the local profile so the data-plane commands pick them up.
func NewAccessKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "access-keys",
		Aliases: []string{"keys", "access-key"},
		GroupID: groupCredentials,
		Short:   "Manage access keys (create, list, rotate, delete)",
		Long: `Manage the S3 access keys of your object storage buckets.

To set up the machine you are on, 'lsh s3 configure' asks the questions and
saves the key for you; the commands here are for keys handed to applications,
CI jobs or other people, and for auditing what exists.

Access keys are separate from your API token: the API creates them, but the
secret is returned once and never again. A key is either fullaccess (every
bucket of the project in that storage class, and site for high_performance)
or limited_access (specific buckets with rw or readonly permission).

Keys can be saved in the active lsh profile (--save, or --save-as <name>);
'lsh s3 copy/ls/rm' then pick the least-privileged saved key that covers the
bucket automatically.
Saved keys never leave this machine except towards the bucket's endpoint.`,
		Example: `  lsh s3 access-keys list
  lsh s3 access-keys list --saved
  lsh s3 access-keys create --bucket backups=rw --bucket logs=readonly --name ci-deploy
  lsh s3 access-keys create --all-buckets --storage-class standard --project my-project --save
  lsh s3 access-keys rotate ci-deploy --delete-old
  lsh s3 access-keys delete ci-deploy --yes
  echo "$SECRET" | lsh s3 access-keys import --name legacy --access-key-id AKIA... --project my-project`,
	}
	cmd.AddCommand(newAccessKeysListCmd())
	cmd.AddCommand(newAccessKeysGetCmd())
	cmd.AddCommand(newAccessKeysCreateCmd())
	cmd.AddCommand(newAccessKeysDeleteCmd())
	cmd.AddCommand(newAccessKeysRotateCmd())
	cmd.AddCommand(newAccessKeysImportCmd())
	cmd.AddCommand(newAccessKeysForgetCmd())
	return cmd
}
