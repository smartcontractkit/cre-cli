package telemetry

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

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

// CollectCommandInfo extracts command information from a cobra command
func CollectCommandInfo(cmd *cobra.Command) CommandInfo {
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
