package storage_objects

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	o := CreateBucketOperation{}
	cmd := &cobra.Command{
		Long: "Create an object storage bucket in a project.\n\n" +
			"Buckets are S3-compatible. Optional flags enable versioning, object\n" +
			"lock (WORM) and a higher-performance storage class where available.\n",
		RunE:  o.run,
		Short: "Create a bucket",
		Example: `  lsh storage-objects create --project my-project --name my-bucket --region SAO2
  lsh storage-objects create --project my-project --name logs --region DAL --storage-class high_performance --versioning`,
		Use: "create",
	}

	cmd.Flags().String("project", "", "Project ID or slug to create the bucket in")
	cmd.Flags().String("name", "", "Bucket name (no special characters or spaces)")
	cmd.Flags().String("region", "", "Site slug representing the region (e.g. DAL, SAO2)")
	cmd.Flags().String("storage-class", "", "Storage tier: standard or high_performance")
	cmd.Flags().Bool("versioning", false, "Enable S3 object versioning")
	cmd.Flags().Bool("locking", false, "Enable S3 Object Lock (WORM); implies versioning")

	return cmd
}

type CreateBucketOperation struct{}

// buildCreateRequest turns parsed flags into the SDK request body. Project,
// name and region are required. Optional toggles are only set when supplied so
// the API defaults apply otherwise. Split out for unit testing.
func buildCreateRequest(project, name, region, storageClass string, versioningSet, versioning, lockingSet, locking bool) (operations.PostStorageBucketsRequestBody, error) {
	if project == "" {
		return operations.PostStorageBucketsRequestBody{}, fmt.Errorf("--project is required")
	}
	if name == "" {
		return operations.PostStorageBucketsRequestBody{}, fmt.Errorf("--name is required")
	}
	if region == "" {
		return operations.PostStorageBucketsRequestBody{}, fmt.Errorf("--region is required")
	}

	attributes := operations.PostStorageBucketsAttributes{
		Project: project,
		Name:    name,
		Region:  region,
	}

	if storageClass != "" {
		switch storageClass {
		case string(operations.StorageClassStandard), string(operations.StorageClassHighPerformance):
			sc := operations.StorageClass(storageClass)
			attributes.StorageClass = &sc
		default:
			return operations.PostStorageBucketsRequestBody{}, fmt.Errorf("invalid --storage-class %q (expected standard or high_performance)", storageClass)
		}
	}
	if versioningSet {
		attributes.Versioning = &versioning
	}
	if lockingSet {
		// The API requires versioning on locked buckets; enforce the documented
		// implication client-side instead of surfacing an opaque server error.
		if locking && versioningSet && !versioning {
			return operations.PostStorageBucketsRequestBody{}, fmt.Errorf("--locking requires versioning; drop --versioning=false or enable it")
		}
		if locking && !versioningSet {
			enabled := true
			attributes.Versioning = &enabled
		}
		attributes.Locking = &locking
	}

	return operations.PostStorageBucketsRequestBody{
		Data: operations.PostStorageBucketsData{
			Type:       operations.PostStorageBucketsTypeObjects,
			Attributes: attributes,
		},
	}, nil
}

func (o *CreateBucketOperation) run(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetString("project")
	name, _ := cmd.Flags().GetString("name")
	region, _ := cmd.Flags().GetString("region")
	storageClass, _ := cmd.Flags().GetString("storage-class")
	versioning, _ := cmd.Flags().GetBool("versioning")
	locking, _ := cmd.Flags().GetBool("locking")

	request, err := buildCreateRequest(
		project, name, region, storageClass,
		cmd.Flags().Changed("versioning"), versioning,
		cmd.Flags().Changed("locking"), locking,
	)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.ObjectStorage.PostStorageBuckets(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object != nil && response.Object.Data != nil {
		fmt.Println(tui.SuccessStyle.Render("✓ Bucket created successfully!"))

		if !lsh.Debug {
			bucket := Bucket{ObjectStorageData: *response.Object.Data}
			utils.RenderStatic(bucket.GetData())
		}
	}

	return nil
}
