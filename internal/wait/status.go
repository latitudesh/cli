package wait

// ServerStatus is the `status` attribute of a server as reported by the API.
// The SDK stopped generating a dedicated enum for it (v1.19.x exposes a plain
// string), so the known values live here to keep callers type-safe.
type ServerStatus string

const (
	ServerStatusOn               ServerStatus = "on"
	ServerStatusOff              ServerStatus = "off"
	ServerStatusUnknown          ServerStatus = "unknown"
	ServerStatusDiskErasing      ServerStatus = "disk_erasing"
	ServerStatusDeploying        ServerStatus = "deploying"
	ServerStatusFailedDeployment ServerStatus = "failed_deployment"
	ServerStatusRescueMode       ServerStatus = "rescue_mode"
)

// VirtualMachineStatus is the `status` attribute of a virtual machine as
// reported by the API (plain string in the SDK since v1.19.x).
type VirtualMachineStatus string

const (
	VirtualMachineStatusRunning            VirtualMachineStatus = "Running"
	VirtualMachineStatusConfiguringNetwork VirtualMachineStatus = "Configuring network"
	VirtualMachineStatusStarting           VirtualMachineStatus = "Starting"
	VirtualMachineStatusScheduling         VirtualMachineStatus = "Scheduling"
	VirtualMachineStatusScheduled          VirtualMachineStatus = "Scheduled"
	VirtualMachineStatusDestroying         VirtualMachineStatus = "Destroying"
)
