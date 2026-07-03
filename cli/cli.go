package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/latitudesh/lsh/client"
	"github.com/latitudesh/lsh/cmd/lsh"
	servers "github.com/latitudesh/lsh/cmd/servers"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/renderer"
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
func makeClient(cmd *cobra.Command, _ []string) (*client.LatitudeShAPI, error) {
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

	// Run ancestor PersistentPreRunE hooks even when a subcommand defines
	// its own. Cobra otherwise runs only the nearest one, which would
	// silently skip the root hook below (profile hydration + project
	// resolution) if a generated command is ever regenerated with its own.
	cobra.EnableTraverseRunHooks = true

	// "Did you mean ..." suggestions for typos in command names.
	rootCmd.SuggestionsMinimumDistance = 2

	// Dedicated group so help topics show up clearly in `lsh --help`.
	rootCmd.AddGroup(&cobra.Group{ID: helpTopicsGroupID, Title: "Help topics:"})
	rootCmd.AddCommand(makeHelpAuthenticationCmd())
	rootCmd.AddCommand(makeHelpProfilesCmd())
	rootCmd.AddCommand(makeHelpAutomationCmd())
	rootCmd.AddCommand(makeHelpOutputFormatsCmd())

	// Re-resolve the active profile once flags have been parsed so that
	// `--profile <name>` overrides LSH_PROFILE / default_profile for the
	// duration of the command. Then resolve the --project flag (env >
	// --all-projects > interactive prompt) for commands that need it.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Record whether --output was set explicitly so the renderer can let an
		// explicit -o (including -o table) win over the --json shortcut.
		if cmd.Flags().Changed("output") {
			viper.Set("output_explicit", true)
		}
		// Validate output/query/pagination selection up front so commands fail
		// fast with an actionable message (and a non-zero exit) instead of
		// silently falling back or clamping.
		if err := renderer.ValidateOutputSelection(); err != nil {
			return err
		}
		if err := pagination.Validate(); err != nil {
			return err
		}
		// Hydrate the active profile into viper for commands that authenticate
		// against the API. Skip the login/auth/profile subtree: there --profile
		// names a profile to create/inspect/remove (it may not exist yet), and
		// those commands handle the flag themselves.
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "" && !managesProfiles(cmd) {
			if err := lsh.HydrateFromActiveProfile(profile); err != nil {
				return err
			}
		}
		return resolveProjectFlag(cmd)
	}

	// Edit commands template
	rootCmd.SetVersionTemplate(fmt.Sprintf("lsh %s\n", rootCmd.Version))

	// register basic flags
	rootCmd.PersistentFlags().String("hostname", client.DefaultHost, "API hostname (override for dev/staging)")
	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	rootCmd.PersistentFlags().String("scheme", client.DefaultSchemes[0], "API scheme (override for dev/staging)")
	viper.BindPFlag("scheme", rootCmd.PersistentFlags().Lookup("scheme"))
	rootCmd.PersistentFlags().String("base-path", client.DefaultBasePath, "API base path (override for dev/staging)")
	viper.BindPFlag("base_path", rootCmd.PersistentFlags().Lookup("base-path"))

	var outputFlag string
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table", "output format: table | json | yaml | csv")
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	// LSH_OUTPUT sets a per-user default format. viper precedence is
	// flag > env > config > default, which is exactly what PD-6072 requires.
	viper.BindEnv("output", "LSH_OUTPUT")

	var formatAsJSON bool
	rootCmd.PersistentFlags().BoolVar(&formatAsJSON, "json", false, "shortcut for --output=json")
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))

	// Global automation controls. --query post-processes structured output with
	// a JMESPath expression; the pagination flags govern every `list` command.
	rootCmd.PersistentFlags().String("query", "", "filter json/yaml/csv output with a JMESPath expression (see 'lsh help output-formats')")
	viper.BindPFlag("query", rootCmd.PersistentFlags().Lookup("query"))

	rootCmd.PersistentFlags().Int64("page-size", pagination.DefaultPageSize, "items to request per API page")
	viper.BindPFlag("page-size", rootCmd.PersistentFlags().Lookup("page-size"))

	rootCmd.PersistentFlags().Int64("max-items", 0, "maximum number of items to return across pages (0 = no limit)")
	viper.BindPFlag("max-items", rootCmd.PersistentFlags().Lookup("max-items"))

	rootCmd.PersistentFlags().Bool("no-paginate", false, "fetch only the first page; print the next page to stderr if more exist")
	viper.BindPFlag("no-paginate", rootCmd.PersistentFlags().Lookup("no-paginate"))

	var noInput bool
	rootCmd.PersistentFlags().BoolVar(&noInput, "no-input", false, "disable interactive prompts; fail fast instead (see 'lsh help automation')")

	rootCmd.PersistentFlags().String("profile", "", "use the named profile (overrides LSH_PROFILE and the stored default)")

	// configure config location
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "path to the lsh config file (default ~/.config/lsh/config.json)")

	// register security flags
	if err := registerAuthInoWriterFlags(rootCmd); err != nil {
		return nil, err
	}

	// add login (browser-assisted by default, with --with-token escape hatch)
	operationLoginCmd, err := makeOperationLoginCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationLoginCmd)

	// `auth` group (status, logout)
	operationAuthCmd, err := makeOperationAuthCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationAuthCmd)

	// `profile` group (use, list) — manages which stored profile is active.
	// Singular form ("profile") to keep the namespace clear vs. `lsh teams`
	// (plural) which manages team resources via the API.
	operationProfileCmd, err := makeOperationProfileCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationProfileCmd)

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

	operationGroupRegionsCmd, err := makeOperationGroupRegionsCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupRegionsCmd)

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

	// The legacy generated `ssh_keys` group is intentionally no longer
	// registered here. It has been replaced by the SDK-based `ssh-keys`
	// (team scope) and `projects ssh-keys` (project scope) groups wired in
	// cmd/build_ssh_keys.go and cmd/build_projects_scoped.go. The kebab-case
	// `ssh-keys` command keeps `ssh_keys` as a hidden alias so the old
	// invocation still works. See PD-6273.

	operationGroupVirtualNetworksCmd, err := makeOperationGroupVirtualNetworksCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupVirtualNetworksCmd)

	operationGroupVolumeCmd, err := makeOperationGroupVolumeCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupVolumeCmd)

	operationGroupTeamsCmd, err := makeOperationGroupTeamsCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupTeamsCmd)

	operationGroupIPsCmd, err := makeOperationGroupIPsCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupIPsCmd)

	operationGroupOperatingSystemsCmd, err := makeOperationGroupOperatingSystemsCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(operationGroupOperatingSystemsCmd)

	// add cobra completion
	rootCmd.AddCommand(makeGenCompletionCmd())

	return rootCmd, nil
}

// registerAuthInoWriterFlags registers all flags needed to perform authentication.
// The --Authorization flag is a low-level escape hatch (it maps directly to the
// HTTP Authorization header) preserved for backwards compatibility. The normal
// path is `lsh login` (browser or --with-token) and the LATITUDESH_TOKEN env var,
// so we keep this flag working but hide it from --help.
func registerAuthInoWriterFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().String("Authorization", "", "raw value for the Authorization header (use `lsh login` or LATITUDESH_TOKEN instead)")
	_ = cmd.PersistentFlags().MarkHidden("Authorization")
	viper.BindPFlag("Authorization", cmd.PersistentFlags().Lookup("Authorization"))
	return nil
}

// managesProfiles reports whether cmd belongs to the login/auth/profile
// subtree. There, --profile names a profile to create, inspect, or remove
// — so it may legitimately not exist yet, and the root hydration hook must
// not require it. Every other command uses --profile to pick an existing
// profile to authenticate with.
func managesProfiles(cmd *cobra.Command) bool {
	top := cmd
	for top.Parent() != nil && top.Parent().HasParent() {
		top = top.Parent()
	}
	switch top.Name() {
	case "login", "auth", "profile":
		return true
	default:
		return false
	}
}

// makeAuthInfoWriter retrieves cmd flags and construct an auth info writer
func makeAuthInfoWriter(_ *cobra.Command) (runtime.ClientAuthInfoWriter, error) {
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
		Use:   "api_keys",
		Short: "Manage API keys",
		Long:  `Commands to manage API keys for authentication`,
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
		Use:   "plans",
		Short: "View server plans",
		Long:  `Commands to view available server plans and their specifications`,
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

	// Add the new enhanced plans list command
	operationGroupPlansCmd.AddCommand(newPlansListCmd())
	operationGroupPlansCmd.AddCommand(newPlansAvailabilityCmd())

	return operationGroupPlansCmd, nil
}
func makeOperationGroupProjectsCmd() (*cobra.Command, error) {
	operationGroupProjectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
		Long:  `Commands to create, list, update and delete projects`,
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
		Use:   "servers",
		Short: "Manage servers",
		Long:  `Commands to create, list, update and delete servers`,
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

	// PD-6075: SDK-backed server extensions.
	operationGroupServersCmd.AddCommand(servers.NewPowerOnCmd())
	operationGroupServersCmd.AddCommand(servers.NewPowerOffCmd())
	operationGroupServersCmd.AddCommand(servers.NewRebootCmd())
	operationGroupServersCmd.AddCommand(servers.NewRescueModeCmd())
	operationGroupServersCmd.AddCommand(servers.NewExitRescueModeCmd())
	operationGroupServersCmd.AddCommand(servers.NewLockCmd())
	operationGroupServersCmd.AddCommand(servers.NewUnlockCmd())

	return operationGroupServersCmd, nil
}

func makeOperationGroupSSHKeysCmd() (*cobra.Command, error) {
	operationGroupSSHKeysCmd := &cobra.Command{
		Use:   "ssh_keys",
		Short: "Manage SSH keys",
		Long:  `Commands to manage SSH keys for server access`,
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
		Use:   "virtual_networks",
		Short: "Manage virtual networks",
		Long:  `Commands to create, list, update and manage virtual networks`,
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

func makeOperationGroupVolumeCmd() (*cobra.Command, error) {
	operationGroupVolumeCmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
		Long:  `Commands to manage volume operations such as listing, mounting, creating, and deleting volumes`,
	}

	operationVolumeListCmd, err := makeOperationVolumeListCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVolumeCmd.AddCommand(operationVolumeListCmd)

	operationVolumeGetCmd, err := makeOperationVolumeGetCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVolumeCmd.AddCommand(operationVolumeGetCmd)

	operationVolumeMountCmd, err := makeOperationVolumeMountCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVolumeCmd.AddCommand(operationVolumeMountCmd)

	operationVolumeCreateCmd, err := makeOperationVolumeCreateCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVolumeCmd.AddCommand(operationVolumeCreateCmd)

	operationVolumeDeleteCmd, err := makeOperationVolumeDeleteCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVolumeCmd.AddCommand(operationVolumeDeleteCmd)

	return operationGroupVolumeCmd, nil
}
