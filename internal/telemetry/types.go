package telemetry

// UserEventInput represents the input for reporting a user event
type UserEventInput struct {
	CliVersion string      `json:"cliVersion"`
	ExitCode   int         `json:"exitCode"`
	Command    CommandInfo `json:"command"`
	Machine    MachineInfo `json:"machine"`
}

// CommandInfo contains information about the executed command
type CommandInfo struct {
	Action     string `json:"action"`
	Subcommand string `json:"subcommand,omitempty"`
}

// MachineInfo contains information about the machine running the CLI
type MachineInfo struct {
	OsName       string `json:"osName"`
	OsVersion    string `json:"osVersion"`
	Architecture string `json:"architecture"`
}

// ReportUserEventResponse represents the response from the reportUserEvent mutation
type ReportUserEventResponse struct {
	ReportUserEvent struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	} `json:"reportUserEvent"`
}
