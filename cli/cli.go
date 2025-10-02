package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/latitudesh/lsh/client"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/version"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// config file location
var configFile string

// name of the executable
var exeName string = filepath.Base(os.Args[0])

// depth of recursion to construct model flags
var maxDepth int = 5

// makeClient constructs a client object
func makeClient(cmd *cobra.Command, args []string) (*client.LatitudeShAPI, error) {
	hostname := viper.GetString("hostname")
	viper.SetDefault("base_path", client.DefaultBasePath)
	basePath := viper.GetString("base_path")
	scheme := viper.GetString("scheme")

	r := httptransport.New(hostname, basePath, []string{scheme})
	r.SetDebug(lsh.Debug)
	// set custom producer and consumer to use the default ones

	r.Consumers["application/json"] = runtime.JSONConsumer()
	r.Consumers["application/vnd.api+json"] = runtime.JSONConsumer()

	r.Producers["application/json"] = runtime.JSONProducer()

	auth, err := makeAuthInfoWriter(cmd)
	if err != nil {
		return nil, err
	}
	r.DefaultAuthentication = auth

	appCli := client.New(r, strfmt.Default)
	lsh.LogDebugf("Server url: %v://%v", scheme, hostname)
	return appCli, nil
}

// MakeRootCmd returns the root cmd
func MakeRootCmd(rootCmd *cobra.Command) (*cobra.Command, error) {
	lsh.InitViperConfigs()

	// Edit commands template
	rootCmd.SetVersionTemplate(fmt.Sprintf("lsh %s\n", rootCmd.Version))

	// register basic flags
	rootCmd.PersistentFlags().String("hostname", client.DefaultHost, "hostname of the service")
	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	rootCmd.PersistentFlags().String("scheme", client.DefaultSchemes[0], fmt.Sprintf("Choose from: %v", client.DefaultSchemes))
	viper.BindPFlag("scheme", rootCmd.PersistentFlags().Lookup("scheme"))
	rootCmd.PersistentFlags().String("base-path", client.DefaultBasePath, fmt.Sprintf("For example: %v", client.DefaultBasePath))
	viper.BindPFlag("base_path", rootCmd.PersistentFlags().Lookup("base-path"))

	var outputFlag string
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table", fmt.Sprintf("For example: %v", "json"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	var formatAsJSON bool
	rootCmd.PersistentFlags().BoolVar(&formatAsJSON, "json", false, "format output as JSON")
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))

	var noInput bool
	rootCmd.PersistentFlags().BoolVar(&noInput, "no-input", false, "skip interactive mode")

	// configure config location
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")

	// register security flags
	if err := registerAuthInoWriterFlags(rootCmd); err != nil {
		return nil, err
	}

	// add login with api -oken
	operationLoginCmd, err := makeOperationLoginCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationLoginCmd)

	operationUpdateCmd, err := makeOperationUpdateCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationUpdateCmd)

	operationGroupAPIKeysCmd, err := makeOperationGroupAPIKeysCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupAPIKeysCmd)

	operationGroupPlansCmd, err := makeOperationGroupPlansCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupPlansCmd)

	operationGroupProjectsCmd, err := makeOperationGroupProjectsCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupProjectsCmd)

	operationGroupServersCmd, err := makeOperationGroupServersCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupServersCmd)

	operationGroupSSHKeysCmd, err := makeOperationGroupSSHKeysCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupSSHKeysCmd)

	operationGroupVirtualNetworksCmd, err := makeOperationGroupVirtualNetworksCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupVirtualNetworksCmd)

	operationGroupBlockCmd, err := makeOperationGroupBlockCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupBlockCmd)

	// add cobra completion
	rootCmd.AddCommand(makeGenCompletionCmd())

	return rootCmd, nil
}

// registerAuthInoWriterFlags registers all flags needed to perform authentication
func registerAuthInoWriterFlags(cmd *cobra.Command) error {
	/*Authorization */
	cmd.PersistentFlags().String("Authorization", "", ``)
	viper.BindPFlag("Authorization", cmd.PersistentFlags().Lookup("Authorization"))
	return nil
}

// makeAuthInfoWriter retrieves cmd flags and construct an auth info writer
func makeAuthInfoWriter(cmd *cobra.Command) (runtime.ClientAuthInfoWriter, error) {
	auths := []runtime.ClientAuthInfoWriter{}
	userAgent := fmt.Sprintf("Latitude-CLI: %s", version.Version)

	/*Authorization */
	if viper.IsSet("Authorization") {
		AuthorizationKey := viper.GetString("Authorization")
		ApiVersion := viper.GetString("api-version")
		auths = append(auths, httptransport.APIKeyAuth("Authorization", "header", AuthorizationKey))
		auths = append(auths, httptransport.APIKeyAuth("API-Version", "header", ApiVersion))
		auths = append(auths, httptransport.APIKeyAuth("User-Agent", "header", userAgent))
	}
	if len(auths) == 0 {
		lsh.LogDebugf("Warning: No auth params detected.")
		return nil, nil
	}
	// compose all auths together
	return httptransport.Compose(auths...), nil
}

func makeOperationGroupAPIKeysCmd() (*cobra.Command, error) {
	operationGroupAPIKeysCmd := &cobra.Command{
		Use:  "api_keys",
		Long: ``,
	}

	operationDeleteAPIKeyCmd, err := makeOperationAPIKeysDeleteAPIKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupAPIKeysCmd.AddCommand(operationDeleteAPIKeyCmd)

	operationGetAPIKeysCmd, err := makeOperationAPIKeysGetAPIKeysCmd()
	if err != nil {
		return nil, err
	}
	operationGroupAPIKeysCmd.AddCommand(operationGetAPIKeysCmd)

	operationPostAPIKeyCmd, err := makeOperationAPIKeysPostAPIKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupAPIKeysCmd.AddCommand(operationPostAPIKeyCmd)

	operationUpdateAPIKeyCmd, err := makeOperationAPIKeysUpdateAPIKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupAPIKeysCmd.AddCommand(operationUpdateAPIKeyCmd)

	return operationGroupAPIKeysCmd, nil
}
func makeOperationGroupPlansCmd() (*cobra.Command, error) {
	operationGroupPlansCmd := &cobra.Command{
		Use:  "plans",
		Long: ``,
	}

	operationGetBandwidthPlansCmd, err := makeOperationPlansGetBandwidthPlansCmd()
	if err != nil {
		return nil, err
	}
	operationGroupPlansCmd.AddCommand(operationGetBandwidthPlansCmd)

	operationGetPlanCmd, err := makeOperationPlansGetPlanCmd()
	if err != nil {
		return nil, err
	}
	operationGroupPlansCmd.AddCommand(operationGetPlanCmd)

	operationGetPlansCmd, err := makeOperationPlansGetPlansCmd()
	if err != nil {
		return nil, err
	}
	operationGroupPlansCmd.AddCommand(operationGetPlansCmd)

	return operationGroupPlansCmd, nil
}
func makeOperationGroupProjectsCmd() (*cobra.Command, error) {
	operationGroupProjectsCmd := &cobra.Command{
		Use:  "projects",
		Long: ``,
	}

	operationCreateProjectCmd, err := makeOperationProjectsCreateProjectCmd()
	if err != nil {
		return nil, err
	}
	operationGroupProjectsCmd.AddCommand(operationCreateProjectCmd)

	operationDeleteProjectCmd, err := makeOperationProjectsDeleteProjectCmd()
	if err != nil {
		return nil, err
	}
	operationGroupProjectsCmd.AddCommand(operationDeleteProjectCmd)

	operationGetProjectCmd, err := makeOperationProjectsGetProjectCmd()
	if err != nil {
		return nil, err
	}
	operationGroupProjectsCmd.AddCommand(operationGetProjectCmd)

	operationGetProjectsCmd, err := makeOperationProjectsGetProjectsCmd()
	if err != nil {
		return nil, err
	}
	operationGroupProjectsCmd.AddCommand(operationGetProjectsCmd)

	operationUpdateProjectCmd, err := makeOperationProjectsUpdateProjectCmd()
	if err != nil {
		return nil, err
	}
	operationGroupProjectsCmd.AddCommand(operationUpdateProjectCmd)

	return operationGroupProjectsCmd, nil
}
func makeOperationGroupServersCmd() (*cobra.Command, error) {
	operationGroupServersCmd := &cobra.Command{
		Use:  "servers",
		Long: ``,
	}

	operationCreateServerCmd, err := makeOperationServersCreateServerCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationCreateServerCmd)

	operationDestroyServerCmd, err := makeOperationServersDestroyServerCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationDestroyServerCmd)

	operationGetServerCmd, err := makeOperationServersGetServerCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationGetServerCmd)

	operationGetServersCmd, err := makeOperationServersGetServersCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationGetServersCmd)

	operationServerScheduleDeletionCmd, err := makeOperationServersServerScheduleDeletionCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationServerScheduleDeletionCmd)

	operationServerUnscheduleDeletionCmd, err := makeOperationServersServerUnscheduleDeletionCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationServerUnscheduleDeletionCmd)

	operationUpdateServerCmd, err := makeOperationServersUpdateServerCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationUpdateServerCmd)

	operationServerReinstallCmd, err := makeOperationServerReinstallCmd()
	if err != nil {
		return nil, err
	}
	operationGroupServersCmd.AddCommand(operationServerReinstallCmd)

	return operationGroupServersCmd, nil
}
func makeOperationGroupSSHKeysCmd() (*cobra.Command, error) {
	operationGroupSSHKeysCmd := &cobra.Command{
		Use:  "ssh_keys",
		Long: ``,
	}

	operationDeleteProjectSSHKeyCmd, err := makeOperationSSHKeysDeleteProjectSSHKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupSSHKeysCmd.AddCommand(operationDeleteProjectSSHKeyCmd)

	operationGetProjectSSHKeyCmd, err := makeOperationSSHKeysGetProjectSSHKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupSSHKeysCmd.AddCommand(operationGetProjectSSHKeyCmd)

	operationGetProjectSSHKeysCmd, err := makeOperationSSHKeysGetProjectSSHKeysCmd()
	if err != nil {
		return nil, err
	}
	operationGroupSSHKeysCmd.AddCommand(operationGetProjectSSHKeysCmd)

	operationPostProjectSSHKeyCmd, err := makeOperationSSHKeysPostProjectSSHKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupSSHKeysCmd.AddCommand(operationPostProjectSSHKeyCmd)

	operationPutProjectSSHKeyCmd, err := makeOperationSSHKeysPutProjectSSHKeyCmd()
	if err != nil {
		return nil, err
	}
	operationGroupSSHKeysCmd.AddCommand(operationPutProjectSSHKeyCmd)

	return operationGroupSSHKeysCmd, nil
}

func makeOperationGroupVirtualNetworksCmd() (*cobra.Command, error) {
	operationGroupVirtualNetworksCmd := &cobra.Command{
		Use:  "virtual_networks",
		Long: ``,
	}

	operationCreateVirtualNetworkCmd, err := makeOperationVirtualNetworksCreateVirtualNetworkCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationCreateVirtualNetworkCmd)

	operationDestroyVirtualNetworkCmd, err := makeOperationVirtualNetworksDestroyVirtualNetworkCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationDestroyVirtualNetworkCmd)

	operationGetVirtualNetworkCmd, err := makeOperationVirtualNetworksGetVirtualNetworkCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationGetVirtualNetworkCmd)

	operationGetVirtualNetworksCmd, err := makeOperationVirtualNetworksGetVirtualNetworksCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationGetVirtualNetworksCmd)

	operationUpdateVirtualNetworkCmd, err := makeOperationVirtualNetworksUpdateVirtualNetworkCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationUpdateVirtualNetworkCmd)

	operationVirtualNetworkAssignmentCmd, err := makeOperationGroupVirtualNetworkAssignmentCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworksCmd.AddCommand(operationVirtualNetworkAssignmentCmd)

	return operationGroupVirtualNetworksCmd, nil
}

func makeOperationGroupBlockCmd() (*cobra.Command, error) {
	operationGroupBlockCmd := &cobra.Command{
		Use:   "block",
		Short: "Manage block storage",
		Long:  `Commands to manage block storage operations such as listing, mounting, and managing`,
	}

	operationBlockListCmd, err := makeOperationBlockListCmd()
	if err != nil {
		return nil, err
	}
	operationGroupBlockCmd.AddCommand(operationBlockListCmd)

	operationBlockGetCmd, err := makeOperationBlockGetCmd()
	if err != nil {
		return nil, err
	}
	operationGroupBlockCmd.AddCommand(operationBlockGetCmd)

	operationBlockMountCmd, err := makeOperationBlockMountCmd()
	if err != nil {
		return nil, err
	}
	operationGroupBlockCmd.AddCommand(operationBlockMountCmd)

	return operationGroupBlockCmd, nil
}
