package telemetry

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/spf13/cobra"
)

// CollectMachineInfo gathers information about the machine running the CLI
func CollectMachineInfo() MachineInfo {
	return MachineInfo{
		OsName:       runtime.GOOS,
		OsVersion:    getOSVersion(),
		Architecture: runtime.GOARCH,
	}
}

// CollectActorInfo returns actor information (only machineId, server populates userId/orgId)
func CollectActorInfo() *ActorInfo {
	// Generate or retrieve machine ID (should be cached/stable)
	// Error is ignored as we always return a machine ID (either system or fallback)
	machineID, _ := getOrCreateMachineID()
	return &ActorInfo{
		MachineID: machineID,
		// userId and organizationId will be populated by the server from the JWT token
	}
}

// CollectWorkflowInfo extracts workflow information from settings
func CollectWorkflowInfo(settings interface{}) *WorkflowInfo {
	// This will be populated by checking if workflow settings exist
	// The exact structure depends on what's available in runtime.Settings
	// For now, return nil as workflow info is optional
	return nil
}

// getOrCreateMachineID retrieves or generates a stable machine ID for telemetry
func getOrCreateMachineID() (string, error) {
	// Try to read existing machine ID from config (for backwards compatibility)
	home, err := os.UserHomeDir()
	if err == nil {
		idFile := fmt.Sprintf("%s/.cre/machine_id", home)
		if data, err := os.ReadFile(idFile); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
	}

	// Use the system machine ID
	machineID, err := machineid.ID()
	if err == nil {
		return fmt.Sprintf("machine_%s", machineID), nil
	}

	// Fallback: generate a simple ID based on hostname
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	fallbackID := fmt.Sprintf("machine_%s_%s_%s", hostname, runtime.GOOS, runtime.GOARCH)
	return fallbackID, fmt.Errorf("failed to get system machine ID, using fallback: %w", err)
}

// CollectCommandInfo extracts command information from a cobra command
func CollectCommandInfo(cmd *cobra.Command, args []string) CommandInfo {
	info := CommandInfo{}

	// Get the action (root command name)
	if cmd.Parent() != nil && cmd.Parent().Name() != "" && cmd.Parent().Name() != "cre" {
		// This is a subcommand, parent is the action
		info.Action = cmd.Parent().Name()
		info.Subcommand = cmd.Name()
	} else if cmd.Name() != "cre" {
		// This is a top-level command
		info.Action = cmd.Name()
	}

	// Collect args (only positional arguments, not flags)
	info.Args = args

	// Omit flags for now - can be added later if needed
	info.Flags = make(map[string]interface{})

	return info
}

// getOSVersion attempts to detect the OS version
func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return getMacOSVersion()
	case "linux":
		return getLinuxVersion()
	case "windows":
		return getWindowsVersion()
	default:
		return "unknown"
	}
}

// getMacOSVersion retrieves macOS version
func getMacOSVersion() string {
	// Try to get version using sw_vers command
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}

	// Fallback: try sysctl
	cmd = exec.Command("sysctl", "-n", "kern.osproductversion")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}

	return "unknown"
}

// getLinuxVersion retrieves Linux version
func getLinuxVersion() string {
	// Try to read /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "VERSION_ID=") {
				version := strings.TrimPrefix(line, "VERSION_ID=")
				version = strings.Trim(version, "\"")
				return version
			}
		}
		// If VERSION_ID not found, try PRETTY_NAME
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				version := strings.TrimPrefix(line, "PRETTY_NAME=")
				version = strings.Trim(version, "\"")
				return version
			}
		}
	}

	// Fallback: try uname
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}

	return "unknown"
}

// getWindowsVersion retrieves Windows version
func getWindowsVersion() string {
	// Try to get version using ver command
	cmd := exec.Command("cmd", "/c", "ver")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}

	return "unknown"
}
