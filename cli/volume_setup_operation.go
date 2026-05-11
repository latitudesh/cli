package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/spf13/cobra"
)

const (
	modulesLoadPath    = "/etc/modules-load.d/nvme-tcp.conf"
	modulesLoadContent = "nvme\nnvme-core\nnvme-tcp\nnvme-fabrics\n"

	modprobePath    = "/etc/modprobe.d/nvme-core.conf"
	modprobeContent = "options nvme_core max_retries=5\n"

	udevRulePath = "/etc/udev/rules.d/71-latitude-block-multipath.rules"
	// The ATTR{model} match is the literal hardware vendor identifier as
	// reported by the kernel; do not change it — udev matches against the
	// hardware-reported string, not user-supplied text.
	udevRuleContent = "ACTION==\"add|change\", SUBSYSTEM==\"nvme-subsystem\", ATTR{model}==\"VASTData\", ATTR{subsystype}==\"nvm\", ATTR{iopolicy}=\"round-robin\"\n"

	discoveryConfPath  = "/etc/nvme/discovery.conf"
	autoconnectService = "nvmf-autoconnect.service"

	defaultGatewayPort = "4420"
)

func makeOperationVolumeSetupCmd() (*cobra.Command, error) {
	operation := VolumeSetupOperation{}
	return operation.Register()
}

type VolumeSetupOperation struct {
	OptionsFlags cmdflag.Flags
}

func (o *VolumeSetupOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure the host for NVMe-oF/TCP volume mounting",
		Long: `One-time, idempotent host configuration for NVMe-oF/TCP block storage.

This command:
  - Persists NVMe kernel modules across reboots (/etc/modules-load.d/nvme-tcp.conf)
  - Tunes nvme_core max_retries (/etc/modprobe.d/nvme-core.conf)
  - Installs the round-robin multipath I/O udev rule
  - Seeds /etc/nvme/discovery.conf with the gateway IP
  - Rebuilds initramfs (Ubuntu: update-initramfs / RHEL: dracut)
  - Enables the nvmf-autoconnect service for reboot-resilient mounts

Run once per server. After this, "lsh volume mount --id <vol>" will reconnect
automatically after reboot with round-robin multipath I/O.

This command must be run with sudo/root privileges.

Example:
  sudo lsh volume setup --gateway-ip 10.0.1.10`,
		RunE:   o.run,
		PreRun: o.preRun,
	}
	o.registerFlags(cmd)
	return cmd, nil
}

func (o *VolumeSetupOperation) registerFlags(cmd *cobra.Command) {
	o.OptionsFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	optionsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "gateway-ip",
			Label:       "Gateway IP",
			Description: "The block storage gateway IP to seed into /etc/nvme/discovery.conf",
			Required:    true,
		},
		&cmdflag.String{
			Name:        "gateway-port",
			Label:       "Gateway Port",
			Description: fmt.Sprintf("NVMe-oF/TCP port (default: %s)", defaultGatewayPort),
			Required:    false,
		},
	}

	o.OptionsFlags.Register(optionsSchema)
}

func (o *VolumeSetupOperation) preRun(cmd *cobra.Command, args []string) {
	o.OptionsFlags.PreRun(cmd, args)
}

func (o *VolumeSetupOperation) run(cmd *cobra.Command, args []string) error {
	if err := checkRoot(); err != nil {
		printError(err.Error())
		return err
	}

	gatewayIP, _ := cmd.Flags().GetString("gateway-ip")
	gatewayPort, _ := cmd.Flags().GetString("gateway-port")
	if gatewayPort == "" {
		gatewayPort = defaultGatewayPort
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip applying host changes.")
		return nil
	}

	fmt.Fprintf(os.Stdout, "\n🔧 Configuring host for NVMe-oF/TCP volume mounting...\n\n")

	if err := checkPrerequisites(); err != nil {
		printError(err.Error())
		return err
	}

	steps := []func() error{
		writeModulesLoadConfig,
		writeModprobeConfig,
		writeUdevRule,
		reloadUdev,
		func() error { return writeDiscoveryConf(gatewayIP, gatewayPort) },
		rebuildInitramfs,
		enableAutoconnect,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			printError(err.Error())
			return err
		}
	}

	fmt.Fprintf(os.Stdout, "\n✅ Host setup complete.\n")
	fmt.Fprintf(os.Stdout, "\nNext step: sudo lsh volume mount --id <volume_id>\n")
	return nil
}

func writeModulesLoadConfig() error {
	printStatus(fmt.Sprintf("Writing %s", modulesLoadPath))
	return writeFileIdempotent(modulesLoadPath, modulesLoadContent, 0644)
}

func writeModprobeConfig() error {
	printStatus(fmt.Sprintf("Writing %s", modprobePath))
	return writeFileIdempotent(modprobePath, modprobeContent, 0644)
}

func writeUdevRule() error {
	printStatus(fmt.Sprintf("Writing %s", udevRulePath))
	return writeFileIdempotent(udevRulePath, udevRuleContent, 0644)
}

func reloadUdev() error {
	printStatus("Reloading udev rules")
	if _, err := runCommand("udevadm", "control", "--reload-rules"); err != nil {
		return fmt.Errorf("udevadm control --reload-rules: %w", err)
	}
	if _, err := runCommand("udevadm", "trigger"); err != nil {
		return fmt.Errorf("udevadm trigger: %w", err)
	}
	return nil
}

func writeDiscoveryConf(gatewayIP, gatewayPort string) error {
	printStatus(fmt.Sprintf("Seeding %s with %s:%s", discoveryConfPath, gatewayIP, gatewayPort))

	if err := os.MkdirAll(filepath.Dir(discoveryConfPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(discoveryConfPath), err)
	}

	line := fmt.Sprintf("--transport=tcp --traddr=%s --trsvcid=%s", gatewayIP, gatewayPort)

	existing, err := os.ReadFile(discoveryConfPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", discoveryConfPath, err)
	}

	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			printStatus("  ✓ discovery entry already present")
			return nil
		}
	}

	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"

	return os.WriteFile(discoveryConfPath, []byte(content), 0644)
}

func rebuildInitramfs() error {
	if _, err := exec.LookPath("update-initramfs"); err == nil {
		printStatus("Rebuilding initramfs (update-initramfs -u)")
		if _, err := runCommand("update-initramfs", "-u"); err != nil {
			return fmt.Errorf("update-initramfs: %w", err)
		}
		return nil
	}
	if _, err := exec.LookPath("dracut"); err == nil {
		printStatus("Rebuilding initramfs (dracut -f)")
		if _, err := runCommand("dracut", "-f"); err != nil {
			return fmt.Errorf("dracut: %w", err)
		}
		return nil
	}
	printWarning("Neither update-initramfs nor dracut found; skipping initramfs rebuild")
	return nil
}

func enableAutoconnect() error {
	printStatus(fmt.Sprintf("Enabling %s", autoconnectService))
	if _, err := runCommand("systemctl", "enable", autoconnectService); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", autoconnectService, err)
	}
	return nil
}

func writeFileIdempotent(path, content string, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		printStatus(fmt.Sprintf("  ✓ %s already up to date", path))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func hostSetupApplied() bool {
	if _, err := os.Stat(modulesLoadPath); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(udevRulePath); os.IsNotExist(err) {
		return false
	}
	return true
}
