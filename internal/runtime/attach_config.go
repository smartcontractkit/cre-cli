package runtime

// AttachConfig selects which runtime attach steps run inside ApplyAttachConfig.
// For steps that are not set, the corresponding work is skipped. Settings-related
// fields are OR'd: if any are true, project settings (settings.New) are loaded
// as a single step — loadWorkflowSettings/CLD are not split in this first iteration.
type AttachConfig struct {
	Environment           bool
	Credentials           bool
	TenantContext         bool
	ExecutionContext      bool
	ConfigMerge           bool
	Target                bool
	WorkflowConfig        bool
	StorageConfig         bool
	CLDConfig             bool
	ValidateDeploymentRPC bool
	ResolvedRegistry      bool
	// ResolveWorkflowOwner: when true with ResolvedRegistry, run settings.FinalizeWorkflowOwner
	// at the end of AttachResolvedRegistry (same pre-run apply step).
	ResolveWorkflowOwner bool
	// SkipCredentialValidation passes through to AttachCredentials.
	SkipCredentialValidation bool
}

// NeedsSettingsLoad returns true when we must run settings.New (Viper + workflow YAML + user + storage + CLD).
func (a *AttachConfig) NeedsSettingsLoad() bool {
	if a == nil {
		return false
	}
	if a.ResolvedRegistry || a.ResolveWorkflowOwner {
		return true
	}
	return a.ConfigMerge || a.Target || a.WorkflowConfig || a.StorageConfig || a.CLDConfig
}

// IsEmpty reports whether Apply would perform no work (all attach flags off).
// Environment is always loaded by root; this type does not track that.
func (a *AttachConfig) IsEmpty() bool {
	if a == nil {
		return true
	}
	if a.Credentials || a.TenantContext || a.ExecutionContext {
		return false
	}
	if a.NeedsSettingsLoad() {
		return false
	}
	if a.ValidateDeploymentRPC {
		return false
	}
	return true
}
