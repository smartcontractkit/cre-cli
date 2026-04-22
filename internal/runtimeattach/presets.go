package runtimeattach

import creruntime "github.com/smartcontractkit/cre-cli/internal/runtime"

// Shared, read-only attach presets. Commands register a pointer; multiple
// commands can share the same pointer when their attach behavior is identical.

// Empty is a no-op in Apply.
var Empty = &creruntime.AttachConfig{}

// CredsAndTenant loads only credentials and tenant context (e.g. whoami, registry list without project settings).
var CredsAndTenant = &creruntime.AttachConfig{
	Credentials:   true,
	TenantContext: true,
}

// ProjectSettingsNoCreds loads project path, Viper settings, and registry/owner
// without loading cloud credentials (offline hash).
var ProjectSettingsNoCreds = &creruntime.AttachConfig{
	ExecutionContext:     true,
	ConfigMerge:          true,
	Target:               true,
	WorkflowConfig:       true,
	StorageConfig:        true,
	CLDConfig:            true,
	ResolvedRegistry:     true,
	ResolveWorkflowOwner: true,
}

// Full loads credentials and full project settings (default for workflow deploy paths that do not validate deployment RPC).
var Full = &creruntime.AttachConfig{
	Credentials:          true,
	TenantContext:        true,
	ExecutionContext:     true,
	ConfigMerge:          true,
	Target:               true,
	WorkflowConfig:       true,
	StorageConfig:        true,
	CLDConfig:            true,
	ResolvedRegistry:     true,
	ResolveWorkflowOwner: true,
}

// FullWithDeploymentRPC is Full plus registry-chain RPC validation for on-chain registries.
var FullWithDeploymentRPC = &creruntime.AttachConfig{
	Credentials:           true,
	TenantContext:         true,
	ExecutionContext:      true,
	ConfigMerge:           true,
	Target:                true,
	WorkflowConfig:        true,
	StorageConfig:         true,
	CLDConfig:             true,
	ResolvedRegistry:      true,
	ResolveWorkflowOwner:  true,
	ValidateDeploymentRPC: true,
}
