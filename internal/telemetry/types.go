package telemetry

// UserEventInput represents the input for reporting a user event
type UserEventInput struct {
	CliVersion   string         `json:"cliVersion"`
	ExitCode     int            `json:"exitCode"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	Command      CommandInfo    `json:"command"`
	Machine      MachineInfo    `json:"machine"`
	Workflow     *WorkflowInfo  `json:"workflow,omitempty"`
	Actor        *ActorInfo     `json:"actor,omitempty"`
	Attributes   []KeyValuePair `json:"attributes,omitempty"`
}

// KeyValuePair represents a key-value pair for flags and attributes
type KeyValuePair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CommandInfo contains information about the executed command
type CommandInfo struct {
	Action     string         `json:"action"`
	Subcommand string         `json:"subcommand,omitempty"`
	Args       []string       `json:"args,omitempty"`
	Flags      []KeyValuePair `json:"flags,omitempty"`
}

// MachineInfo contains information about the machine running the CLI
type MachineInfo struct {
	OsName       string `json:"osName"`
	OsVersion    string `json:"osVersion"`
	Architecture string `json:"architecture"`
}

// WorkflowInfo contains information about the workflow being operated on
type WorkflowInfo struct {
	OwnerAddress string `json:"ownerAddress,omitempty"`
	Name         string `json:"name,omitempty"`
	ID           string `json:"id,omitempty"`
	Language     string `json:"language,omitempty"`
}

// ActorInfo contains information about the actor performing the action
type ActorInfo struct {
	UserID         string `json:"userId,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
	MachineID      string `json:"machineId"`
}

// ReportUserEventResponse represents the response from the reportUserEvent mutation
type ReportUserEventResponse struct {
	ReportUserEvent struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	} `json:"reportUserEvent"`
}
