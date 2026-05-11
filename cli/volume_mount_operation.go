package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorReset  = "\033[0m"
)

func makeOperationVolumeMountCmd() (*cobra.Command, error) {
	operation := VolumeMountOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type VolumeMountOperation struct {
	PathParamFlags cmdflag.Flags
	OptionsFlags   cmdflag.Flags
}

func (o *VolumeMountOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "mount",
		Short: "Mount a volume storage to a server",
		Long: `Mount a block storage volume to a server. This command will:
  1. Auto-detect the server's NQN from /etc/nvme/hostnqn
     (or generate a new one if the file doesn't exist)
  2. Send the client NQN to the API to authorize access
  3. Receive the subsystem NQN, namespace ID, and gateway VIPs back from the API
  4. Seed /etc/nvme/discovery.conf with the gateway VIPs
  5. Run "nvme connect-all" to attach the volume

This command must be run with sudo/root privileges on the target server.

Example:
  sudo lsh volume mount --id vol_abc123`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *VolumeMountOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.OptionsFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Volume Storage ID",
			Description: "The ID of the volume storage to mount",
			Required:    true,
		},
	}

	optionsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "nqn",
			Label:       "NVMe Qualified Name (NQN)",
			Description: "NVMe Qualified Name of the server (will auto-detect if not provided)",
			Required:    false,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.OptionsFlags.Register(optionsSchema)
}

func (o *VolumeMountOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
	o.OptionsFlags.PreRun(cmd, args)
}

func printStatus(msg string) {
	fmt.Fprintf(os.Stdout, "%s[INFO]%s %s\n", colorGreen, colorReset, msg)
}

func printWarning(msg string) {
	fmt.Fprintf(os.Stdout, "%s[WARN]%s %s\n", colorYellow, colorReset, msg)
}

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorReset, msg)
}

// checkRoot verifies if the command is running as root
func checkRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf(`this command must be run as root (use sudo)

This command requires root privileges to:
- Install nvme-cli if not present
- Load NVMe kernel modules
- Configure NVMe host settings
- Connect to NVMe-oF targets

Usage:
  sudo lsh volume mount --id <VOLUME_ID>

Note: Your API key will be automatically detected from your user config,
      so make sure you've logged in first:
  lsh login <API_KEY>`)
	}
	return nil
}

// getHostNQN attempts to read the host NQN from /etc/nvme/hostnqn
// If the file doesn't exist, it generates a new NQN and creates the file
func getHostNQN() (string, error) {
	// Try to read existing NQN
	content, err := os.ReadFile("/etc/nvme/hostnqn")
	if err == nil {
		nqn := strings.TrimSpace(string(content))
		if nqn != "" {
			return nqn, nil
		}
	}

	// File doesn't exist or is empty - generate a new NQN
	printWarning("/etc/nvme/hostnqn not found or empty, generating new NQN...")

	// Generate NQN using nvme-cli
	cmd := exec.Command("nvme", "gen-hostnqn")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate NQN (is nvme-cli installed?): %w", err)
	}

	nqn := strings.TrimSpace(string(output))
	if nqn == "" {
		return "", fmt.Errorf("generated NQN is empty")
	}

	printStatus(fmt.Sprintf("Generated new NQN: %s", nqn))

	// Create directory if it doesn't exist
	if err := os.MkdirAll("/etc/nvme", 0755); err != nil {
		return "", fmt.Errorf("failed to create /etc/nvme directory: %w", err)
	}

	// Write the NQN to file
	if err := os.WriteFile("/etc/nvme/hostnqn", []byte(nqn+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed to write /etc/nvme/hostnqn: %w", err)
	}

	printStatus("Created /etc/nvme/hostnqn with new NQN")

	return nqn, nil
}

// ensureHostNQN ensures /etc/nvme/hostnqn exists and contains the correct NQN
func ensureHostNQN(nqn string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll("/etc/nvme", 0755); err != nil {
		return fmt.Errorf("failed to create /etc/nvme directory: %w", err)
	}

	// Check if file exists and has the correct NQN
	if content, err := os.ReadFile("/etc/nvme/hostnqn"); err == nil {
		currentNQN := strings.TrimSpace(string(content))
		if currentNQN == nqn {
			printStatus("Host NQN already configured correctly")
			return nil
		}
		printWarning(fmt.Sprintf("Updating host NQN from %s to %s", currentNQN, nqn))
	}

	// Write the NQN
	if err := os.WriteFile("/etc/nvme/hostnqn", []byte(nqn+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/nvme/hostnqn: %w", err)
	}

	printStatus("Host NQN configured successfully")
	return nil
}

// runCommand executes a shell command and returns the output
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// installNvmeCli attempts to auto-install nvme-cli based on the OS
func installNvmeCli() error {
	printWarning("nvme-cli is not installed. Attempting to install...")

	// Try apt (Ubuntu/Debian)
	if _, err := exec.LookPath("apt"); err == nil {
		printStatus("Detected apt package manager (Ubuntu/Debian)")
		printStatus("Running: apt update && apt install -y nvme-cli")

		// Update package list
		if _, err := runCommand("apt", "update"); err != nil {
			return fmt.Errorf("failed to update apt: %w", err)
		}

		// Install nvme-cli
		if _, err := runCommand("apt", "install", "-y", "nvme-cli"); err != nil {
			return fmt.Errorf("failed to install nvme-cli via apt: %w", err)
		}

		printStatus("✓ nvme-cli installed successfully via apt")
		return nil
	}

	// Try yum (CentOS/RHEL)
	if _, err := exec.LookPath("yum"); err == nil {
		printStatus("Detected yum package manager (CentOS/RHEL)")
		printStatus("Running: yum install -y nvme-cli")

		if _, err := runCommand("yum", "install", "-y", "nvme-cli"); err != nil {
			return fmt.Errorf("failed to install nvme-cli via yum: %w", err)
		}

		printStatus("✓ nvme-cli installed successfully via yum")
		return nil
	}

	// Try dnf (Fedora/newer RHEL)
	if _, err := exec.LookPath("dnf"); err == nil {
		printStatus("Detected dnf package manager (Fedora/newer RHEL)")
		printStatus("Running: dnf install -y nvme-cli")

		if _, err := runCommand("dnf", "install", "-y", "nvme-cli"); err != nil {
			return fmt.Errorf("failed to install nvme-cli via dnf: %w", err)
		}

		printStatus("✓ nvme-cli installed successfully via dnf")
		return nil
	}

	return fmt.Errorf("could not detect package manager (apt/yum/dnf). Please install nvme-cli manually")
}

// checkPrerequisites verifies that all required tools are installed
func checkPrerequisites() error {
	printStatus("Checking prerequisites...")

	// Check if nvme-cli is installed, if not try to install it
	if _, err := exec.LookPath("nvme"); err != nil {
		if err := installNvmeCli(); err != nil {
			return fmt.Errorf(`nvme-cli installation failed: %w
Please install manually:
  Ubuntu/Debian: sudo apt install nvme-cli
  CentOS/RHEL: sudo yum install nvme-cli`, err)
		}
	} else {
		printStatus("✓ nvme-cli is installed")
	}

	// Load NVMe modules. nvme-tcp depends on nvme-fabrics and nvme-core, which
	// the kernel will auto-pull; we modprobe nvme + nvme-tcp explicitly so a
	// missing module surfaces as an error rather than silently failing later.
	printStatus("Loading NVMe-oF TCP modules...")
	for _, mod := range []string{"nvme", "nvme-tcp"} {
		if _, err := runCommand("modprobe", mod); err != nil {
			printWarning(fmt.Sprintf("modprobe %s may already be loaded", mod))
		}
	}

	// Check multipath setting (informational)
	if multipathStatus, err := runCommand("cat", "/sys/module/nvme_core/parameters/multipath"); err == nil {
		printStatus(fmt.Sprintf("NVMe multipath is: %s", multipathStatus))
	}

	return nil
}

// testConnectivity validates the NVMe-oF/TCP path to the gateway by running
// `nvme discover`. Block storage gateways may not respond to ICMP, so a plain
// ping is an unreliable signal; a successful discover both proves L4
// reachability and confirms the gateway is willing to expose subsystems to
// this host.
func testConnectivity(gatewayIP, gatewayPort string) error {
	printStatus(fmt.Sprintf("Probing %s:%s with nvme discover...", gatewayIP, gatewayPort))

	if _, err := runCommand("nvme", "discover", "-t", "tcp", "-a", gatewayIP, "-s", gatewayPort); err != nil {
		return fmt.Errorf("nvme discover to %s:%s failed: %w", gatewayIP, gatewayPort, err)
	}

	printStatus("Gateway is reachable")
	return nil
}

// disconnectExisting disconnects any existing connection to the subsystem
func disconnectExisting(subsystemNQN string) {
	printStatus("Checking for existing connections...")

	output, err := runCommand("nvme", "list-subsys")
	if err != nil {
		// nvme list-subsys might fail if no devices, that's ok
		return
	}

	if strings.Contains(output, subsystemNQN) {
		printWarning("Already connected. Disconnecting...")
		runCommand("nvme", "disconnect", "-n", subsystemNQN)
		time.Sleep(2 * time.Second)
	}
}

// verifyConnection verifies the connection and shows available devices
func verifyConnection(subsystemNQN string) error {
	printStatus("Verifying connection...")
	time.Sleep(3 * time.Second)

	// Check if subsystem is connected
	output, err := runCommand("nvme", "list-subsys")
	if err != nil || !strings.Contains(output, subsystemNQN) {
		return fmt.Errorf("subsystem not found after connection")
	}

	// Find the NVMe device (dynamically detects nvme0, nvme1, nvme2, etc.)
	lines := strings.Split(output, "\n")
	var nvmeDevice string
	for _, line := range lines {
		// Look for lines with NVMe controller info (e.g., " +- nvme1 tcp traddr=...")
		if strings.Contains(line, "nvme") {
			fields := strings.Fields(line)
			for _, field := range fields {
				// Find field that starts with "nvme" followed by a number
				if strings.HasPrefix(field, "nvme") && !strings.Contains(field, "/") && !strings.Contains(field, "-") {
					nvmeDevice = field
					break
				}
			}
			if nvmeDevice != "" {
				break // Found it, stop searching
			}
		}
	}

	if nvmeDevice == "" {
		printError("Could not detect NVMe controller from nvme list-subsys")
		printWarning("Output was:")
		fmt.Fprintf(os.Stderr, "%s\n", output)
		return fmt.Errorf("could not find NVMe device - check if connection succeeded")
	}

	printStatus(fmt.Sprintf("✓ Detected NVMe controller: %s", nvmeDevice))

	// Check for block devices using find (e.g., /dev/nvme0n1, /dev/nvme1n1, etc.)
	blockDevices, _ := runCommand("find", "/dev", "-name", fmt.Sprintf("%sn*", nvmeDevice))
	if blockDevices != "" {
		printStatus("Volume devices available:")
		devices := strings.Split(blockDevices, "\n")
		var validDevices []string
		for _, dev := range devices {
			dev = strings.TrimSpace(dev)
			if dev != "" {
				validDevices = append(validDevices, dev)
				fmt.Fprintf(os.Stdout, "  %s\n", dev)
			}
		}

		if len(validDevices) > 0 {
			fmt.Fprintf(os.Stdout, "\n")
			printStatus("To use the volume, format and mount it. For example:")
			dev := validDevices[0] // Use first device
			deviceName := strings.TrimPrefix(dev, "/dev/")
			mountpoint := fmt.Sprintf("/mnt/%s", deviceName)
			fmt.Fprintf(os.Stdout, "  sudo mkfs.ext4 %s\n", dev)
			fmt.Fprintf(os.Stdout, "  sudo mkdir -p %s\n", mountpoint)
			fmt.Fprintf(os.Stdout, "  sudo mount %s %s\n\n", dev, mountpoint)
		}
	} else {
		printWarning("No devices found. The volume may not be accessible yet.")
		printWarning("Wait a few seconds and check: sudo nvme list")
	}

	return nil
}

func (o *VolumeMountOperation) run(cmd *cobra.Command, args []string) error {
	// Check if running as root
	if err := checkRoot(); err != nil {
		printError(err.Error())
		return err
	}

	if !hostSetupApplied() {
		printWarning("Host is not production-configured (missing module persistence and/or multipath udev rule).")
		printWarning("Run 'sudo lsh volume setup' for reboot-resilient mounts and round-robin multipath I/O.")
		printWarning("Continuing with one-shot mount...")
	}

	// Get the volume ID from flags
	volumeID, err := cmd.Flags().GetString("id")
	if err != nil {
		return fmt.Errorf("error getting volume ID: %w", err)
	}

	fmt.Fprintf(os.Stdout, "\n🔧 Preparing server for volume mount...\n\n")

	// STEP 1: Install prerequisites (nvme-cli) BEFORE getting NQN
	if err := checkPrerequisites(); err != nil {
		printError(err.Error())
		return err
	}

	// STEP 2: Get NQN (now that nvme-cli is installed)
	nqnFlag, _ := cmd.Flags().GetString("nqn")
	var nqn string

	if nqnFlag != "" {
		nqn = nqnFlag
		printStatus(fmt.Sprintf("Using provided NQN: %s", nqn))
	} else {
		// Try to auto-detect or generate (nvme-cli is now guaranteed to be installed)
		printStatus("Getting server NQN...")
		detectedNQN, err := getHostNQN()
		if err != nil {
			printError(fmt.Sprintf("Could not get or generate NQN: %v", err))
			printError("\nOr provide NQN manually:")
			printError(fmt.Sprintf("  sudo lsh volume mount --id %s --nqn nqn.2014-08.org.nvmexpress:uuid:YOUR-UUID", volumeID))
			return fmt.Errorf("NQN is required but could not be obtained")
		}
		nqn = detectedNQN
		printStatus(fmt.Sprintf("✓ Using NQN: %s", nqn))
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	// Get API key - try both "authorization" and "Authorization" for compatibility
	apiKey := viper.GetString("authorization")
	if apiKey == "" {
		apiKey = viper.GetString("Authorization")
	}
	if apiKey == "" {
		return fmt.Errorf("API key not found. Please run 'lsh login <API_KEY>' first")
	}

	fmt.Fprintf(os.Stdout, "\n📦 Authorizing client and mounting volume...\n")
	printStatus(fmt.Sprintf("Volume ID: %s", volumeID))
	printStatus(fmt.Sprintf("Client NQN (for authorization): %s", nqn))

	if lsh.Debug {
		fmt.Fprintf(os.Stdout, "[DEBUG] POST /storage/volumes/%s/mount (Api-Version: %s)\n", volumeID, v4APIVersion)
		fmt.Fprintf(os.Stdout, "[DEBUG] Request body NQN: %s\n", nqn)
	}

	apiClient := newV4Client(apiKey)
	volume, err := apiClient.MountVolume(volumeID, nqn)
	if err != nil {
		printError(fmt.Sprintf("API call failed: %v", err))
		return err
	}

	subsystemNQN := volume.Attributes.SubsystemNQN
	if subsystemNQN == "" {
		return fmt.Errorf("API response missing subsystem_nqn — the volume may not be on a current-generation backend")
	}
	if len(volume.Attributes.GatewayVIPs) == 0 {
		return fmt.Errorf("API response missing gateway_vips — the volume may not be on a current-generation backend")
	}

	printStatus(fmt.Sprintf("✓ Subsystem NQN: %s", subsystemNQN))
	printStatus(fmt.Sprintf("✓ Gateway VIPs:  %s", strings.Join(volume.Attributes.GatewayVIPs, ", ")))
	if volume.Attributes.NamespaceID != nil {
		printStatus(fmt.Sprintf("✓ Namespace ID:  %d", *volume.Attributes.NamespaceID))
	}

	fmt.Fprintf(os.Stdout, "\n📡 Connecting to NVMe-oF storage...\n\n")

	if err := ensureHostNQN(nqn); err != nil {
		printError(fmt.Sprintf("Failed to ensure host NQN: %v", err))
		return err
	}

	// Seed /etc/nvme/discovery.conf with every VIP the API returned. The
	// helper is idempotent: if a line already exists (e.g. from a previous
	// mount of another volume in the same pool, or from `lsh volume setup`),
	// it is left in place. This also ensures nvmf-autoconnect.service has
	// a complete discovery seed for reboot reconnection.
	for _, vip := range volume.Attributes.GatewayVIPs {
		if err := writeDiscoveryConf(vip, "4420"); err != nil {
			printError(fmt.Sprintf("Failed to update %s: %v", discoveryConfPath, err))
			return err
		}
	}

	// Fast-fail check against the first VIP before attempting connect-all.
	if err := testConnectivity(volume.Attributes.GatewayVIPs[0], "4420"); err != nil {
		printError(fmt.Sprintf("Connectivity test failed: %v", err))
		return err
	}

	disconnectExisting(subsystemNQN)

	printStatus("Running `nvme connect-all` (multipath fan-out reads /etc/nvme/discovery.conf)...")
	if _, err := runCommand("nvme", "connect-all"); err != nil {
		printError(fmt.Sprintf("nvme connect-all failed: %v", err))
		return fmt.Errorf("nvme connect-all failed: %w", err)
	}
	printStatus("✓ Connected")

	if err := verifyConnection(subsystemNQN); err != nil {
		printError(fmt.Sprintf("Connection verification failed: %v", err))
		return err
	}

	fmt.Fprintf(os.Stdout, "\n✅ Volume mount complete!\n")
	fmt.Fprintf(os.Stdout, "\nConnection Summary:\n")
	fmt.Fprintf(os.Stdout, "  Client NQN:    %s\n", nqn)
	fmt.Fprintf(os.Stdout, "  Subsystem NQN: %s\n", subsystemNQN)
	fmt.Fprintf(os.Stdout, "  Gateway VIPs:  %s\n", strings.Join(volume.Attributes.GatewayVIPs, ", "))

	return nil
}
