package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorReset  = "\033[0m"
)

func makeOperationBlockMountCmd() (*cobra.Command, error) {
	operation := BlockMountOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type BlockMountOperation struct {
	PathParamFlags cmdflag.Flags
	OptionsFlags   cmdflag.Flags
}

func (o *BlockMountOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "mount",
		Short: "Mount a block storage to a server",
		Long: `Mount a block storage to a server. This command will:
  1. Auto-fetch block storage details (including connector_id)
  2. Auto-detect the server's NQN from /etc/nvme/hostnqn
     (or generate a new one if the file doesn't exist)
  3. Send the client NQN to the API to authorize access
  4. Execute all NVMe-oF connection steps automatically

The mount process:
- Block ID: Used to fetch connector_id (subsystem NQN) automatically
- Client NQN (--nqn or auto-detected): Sent to API to authorize this client
- Subsystem NQN: Auto-fetched from block storage's connector_id
- Gateway: The NVMe-oF gateway IP and port (defaults to 67.213.118.147:4420)

This command must be run with sudo/root privileges on the target server.

Example:
  sudo lsh block mount --id blk_abc123`,
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *BlockMountOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.OptionsFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Block Storage ID",
			Description: "The ID of the block storage to mount",
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
		&cmdflag.String{
			Name:        "gateway-ip",
			Label:       "Gateway IP",
			Description: "Override the gateway IP address (optional, default: 67.213.118.147)",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "gateway-port",
			Label:       "Gateway Port",
			Description: "Override the gateway port (optional, default: 4420)",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "subsystem-nqn",
			Label:       "Subsystem NQN",
			Description: "Override the subsystem NQN (optional, auto-fetched from block storage's connector_id)",
			Required:    false,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.OptionsFlags.Register(optionsSchema)
}

func (o *BlockMountOperation) preRun(cmd *cobra.Command, args []string) {
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
		return fmt.Errorf("this command must be run as root (use sudo)")
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

	// Load NVMe TCP module
	printStatus("Loading NVMe-oF TCP module...")
	if _, err := runCommand("modprobe", "nvme_tcp"); err != nil {
		printWarning("nvme_tcp module may already be loaded")
	}

	// Check multipath setting (informational)
	if multipathStatus, err := runCommand("cat", "/sys/module/nvme_core/parameters/multipath"); err == nil {
		printStatus(fmt.Sprintf("NVMe multipath is: %s", multipathStatus))
	}

	return nil
}

// testConnectivity tests network connectivity to the gateway
func testConnectivity(gatewayIP string) error {
	printStatus(fmt.Sprintf("Testing connectivity to %s...", gatewayIP))

	cmd := exec.Command("ping", "-c", "2", "-W", "2", gatewayIP)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot reach gateway at %s", gatewayIP)
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

// connectNVMeoF connects to the NVMe-oF target
func connectNVMeoF(gatewayIP, gatewayPort, subsystemNQN string) error {
	printStatus("Connecting to NVMe-oF target...")
	printStatus(fmt.Sprintf("  Gateway: %s:%s", gatewayIP, gatewayPort))
	printStatus(fmt.Sprintf("  Subsystem: %s", subsystemNQN))

	_, err := runCommand("nvme", "connect", "-t", "tcp", "-a", gatewayIP, "-s", gatewayPort, "-n", subsystemNQN)
	if err != nil {
		return fmt.Errorf(`connection failed. Please check:
  1. Gateway is accessible from this server
  2. Client NQN is authorized on the gateway
  3. Block storage is properly configured`)
	}

	printStatus("Successfully connected!")
	return nil
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

	// Find the NVMe device
	lines := strings.Split(output, "\n")
	var nvmeDevice string
	for _, line := range lines {
		if strings.Contains(line, "nvme") && strings.Contains(line, " nvme") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "nvme") && !strings.Contains(field, "/") {
					nvmeDevice = field
					break
				}
			}
		}
	}

	if nvmeDevice == "" {
		return fmt.Errorf("could not find NVMe device")
	}

	printStatus(fmt.Sprintf("NVMe controller: %s", nvmeDevice))

	// Check for block devices
	blockDevices, _ := runCommand("ls", "-1", fmt.Sprintf("/dev/%sn*", nvmeDevice))
	if blockDevices != "" {
		printStatus("Block devices available:")
		devices := strings.Split(blockDevices, "\n")
		for _, dev := range devices {
			if dev != "" {
				fmt.Fprintf(os.Stdout, "  %s\n", dev)
			}
		}

		fmt.Fprintf(os.Stdout, "\n")
		printStatus("To use the block storage, format and mount it. For example:")
		for _, dev := range devices {
			if dev != "" {
				mountpoint := fmt.Sprintf("/mnt/%s", strings.TrimPrefix(dev, "/dev/"))
				fmt.Fprintf(os.Stdout, "  sudo mkfs.ext4 %s\n", dev)
				fmt.Fprintf(os.Stdout, "  sudo mkdir -p %s\n", mountpoint)
				fmt.Fprintf(os.Stdout, "  sudo mount %s %s\n\n", dev, mountpoint)
			}
		}
	} else {
		printWarning("No block devices found. The storage may not be accessible yet.")
		printWarning("Check the ANA (Asymmetric Namespace Access) state on the gateway.")
	}

	return nil
}

func (o *BlockMountOperation) run(cmd *cobra.Command, args []string) error {
	// Check if running as root
	if err := checkRoot(); err != nil {
		printError(err.Error())
		return err
	}

	// Get the block ID from flags
	blockID, err := cmd.Flags().GetString("id")
	if err != nil {
		return fmt.Errorf("error getting block ID: %w", err)
	}

	fmt.Fprintf(os.Stdout, "\n🔧 Preparing server for block storage mount...\n\n")

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
			printError(fmt.Sprintf("  sudo lsh block mount --id %s --nqn nqn.2014-08.org.nvmexpress:uuid:YOUR-UUID", blockID))
			return fmt.Errorf("NQN is required but could not be obtained")
		}
		nqn = detectedNQN
		printStatus(fmt.Sprintf("✓ Using NQN: %s", nqn))
	}

	if dryRun {
		logDebugf("dry-run flag specified. Skip sending request.")
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

	// Initialize the new SDK client
	ctx := context.Background()
	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity(apiKey),
	)

	// Step 1: Fetch block storage details to get connector_id (subsystem NQN)
	subsystemNQN, _ := cmd.Flags().GetString("subsystem-nqn")

	if subsystemNQN == "" {
		// Auto-fetch connector_id from API
		fmt.Fprintf(os.Stdout, "\n📋 Fetching block storage details...\n")
		printStatus(fmt.Sprintf("Block ID: %s", blockID))

		if debug {
			fmt.Fprintf(os.Stdout, "[DEBUG] Fetching block storage details to get connector_id\n")
		}

		blocksResponse, err := client.Storage.GetStorageBlocks(ctx, nil)
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch block storage details: %v", err))
			utils.PrintError(err)
			return err
		}

		// Parse response body manually to get block data
		if blocksResponse != nil && blocksResponse.HTTPMeta.Response != nil {
			bodyBytes, err := io.ReadAll(blocksResponse.HTTPMeta.Response.Body)
			if err != nil {
				printError(fmt.Sprintf("Failed to read response body: %v", err))
				return err
			}

			// Parse JSON response
			var responseData struct {
				Data []struct {
					ID         string `json:"id"`
					Type       string `json:"type"`
					Attributes struct {
						ConnectorID *string `json:"connector_id"`
					} `json:"attributes"`
				} `json:"data"`
			}

			if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
				printError(fmt.Sprintf("Failed to parse response: %v", err))
				return err
			}

			// Find the block by ID
			var found bool
			for _, block := range responseData.Data {
				if block.ID == blockID {
					if block.Attributes.ConnectorID != nil && *block.Attributes.ConnectorID != "" {
						subsystemNQN = *block.Attributes.ConnectorID
						printStatus(fmt.Sprintf("✓ Retrieved connector_id (subsystem NQN): %s", subsystemNQN))
						found = true
						break
					} else {
						printError("Block storage does not have a connector_id configured")
						printError("The block storage must have a connector_id before mounting")
						return fmt.Errorf("connector_id not found for block storage %s", blockID)
					}
				}
			}

			if !found {
				printError(fmt.Sprintf("Block storage not found: %s", blockID))
				return fmt.Errorf("block storage %s not found", blockID)
			}
		} else {
			printError("No response from API")
			return fmt.Errorf("failed to get response from API")
		}
	} else {
		printStatus(fmt.Sprintf("Using provided subsystem NQN: %s", subsystemNQN))
	}

	fmt.Fprintf(os.Stdout, "\n📦 Authorizing client and mounting block storage...\n")
	printStatus(fmt.Sprintf("Block ID: %s", blockID))
	printStatus(fmt.Sprintf("Client NQN (for authorization): %s", nqn))

	if debug {
		fmt.Fprintf(os.Stdout, "[DEBUG] API Request: POST /storage/blocks/%s/mount\n", blockID)
		fmt.Fprintf(os.Stdout, "[DEBUG] Request Body: {\"data\":{\"type\":\"blocks\",\"attributes\":{\"nqn\":\"%s\"}}}\n", nqn)
	}

	// Call the API to authorize the client NQN and mount
	// The NQN authorizes this client to access the storage
	// The subsystem-nqn (connector_id) defines which storage subsystem to connect to
	response, err := client.Storage.PostStorageBlocksMount(ctx, blockID, operations.PostStorageBlocksMountRequestBody{
		Data: operations.PostStorageBlocksMountData{
			Type: operations.PostStorageBlocksMountTypeBlocks,
			Attributes: operations.PostStorageBlocksMountAttributes{
				Nqn: nqn, // Send client NQN to authorize
			},
		},
	})
	if err != nil {
		printError(fmt.Sprintf("API call failed: %v", err))
		utils.PrintError(err)
		return err
	}

	if debug {
		fmt.Fprintf(os.Stdout, "[DEBUG] API Response Status: %d\n", response.HTTPMeta.Response.StatusCode)
	}

	if response != nil && response.HTTPMeta.Response != nil {
		if response.HTTPMeta.Response.StatusCode == 204 || response.HTTPMeta.Response.StatusCode == 200 {
			printStatus("✓ Successfully authorized client and mounted block storage!")
		} else {
			printWarning(fmt.Sprintf("Unexpected status code: %d", response.HTTPMeta.Response.StatusCode))
		}
	} else {
		printWarning("No response from API")
	}

	// Get override values or use defaults
	gatewayIP, _ := cmd.Flags().GetString("gateway-ip")
	gatewayPort, _ := cmd.Flags().GetString("gateway-port")

	// Hardcoded gateway for now
	if gatewayIP == "" {
		gatewayIP = "67.213.118.147" // Hardcoded gateway IP
		printStatus(fmt.Sprintf("Using default gateway IP: %s", gatewayIP))
	}

	if gatewayPort == "" {
		gatewayPort = "4420" // Default NVMe-oF port
	}

	fmt.Fprintf(os.Stdout, "\n📡 Connecting to NVMe-oF storage...\n\n")
	printStatus(fmt.Sprintf("Gateway: %s:%s", gatewayIP, gatewayPort))
	printStatus(fmt.Sprintf("Subsystem NQN: %s", subsystemNQN))

	// Execute mount steps (prerequisites already checked)
	if err := ensureHostNQN(nqn); err != nil {
		printError(fmt.Sprintf("Failed to ensure host NQN: %v", err))
		return err
	}

	if err := testConnectivity(gatewayIP); err != nil {
		printError(fmt.Sprintf("Connectivity test failed: %v", err))
		return err
	}

	disconnectExisting(subsystemNQN)

	if err := connectNVMeoF(gatewayIP, gatewayPort, subsystemNQN); err != nil {
		printError(fmt.Sprintf("NVMe-oF connection failed: %v", err))
		return err
	}

	if err := verifyConnection(subsystemNQN); err != nil {
		printError(fmt.Sprintf("Connection verification failed: %v", err))
		return err
	}

	fmt.Fprintf(os.Stdout, "\n✅ Block storage mount complete!\n")
	fmt.Fprintf(os.Stdout, "\nConnection Summary:\n")
	fmt.Fprintf(os.Stdout, "  Client NQN: %s\n", nqn)
	fmt.Fprintf(os.Stdout, "  Subsystem NQN: %s\n", subsystemNQN)
	fmt.Fprintf(os.Stdout, "  Gateway: %s:%s\n", gatewayIP, gatewayPort)

	return nil
}
