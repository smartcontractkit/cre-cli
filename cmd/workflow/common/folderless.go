package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	crecontext "github.com/smartcontractkit/cre-cli/internal/context"
	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// FolderlessTarget bundles the resolution result for pause/delete invocations
// that run without a project folder: the platform workflow row plus the
// registry the workflow is deployed to.
type FolderlessTarget struct {
	Workflow workflowresolve.Workflow
	Registry settings.ResolvedRegistry
}

// IsFolderlessInvocation reports whether a pause/delete invocation should resolve
// the workflow remotely (by name, ID, or picker) instead of loading project and
// workflow settings from disk. An argument that is an existing directory — or a
// path-like argument — keeps the legacy project-based flow.
//
// The legacy flow resolves the workflow directory relative to the project root
// (found via projectRootFlag or by walking up from the current directory), so the
// directory check must look there as well as at the invocation directory.
func IsFolderlessInvocation(args []string, projectRootFlag string) bool {
	if len(args) == 0 {
		return true
	}
	arg := args[0]
	// Path-like arguments always keep the legacy flow, so the user gets the
	// original "path not found" error rather than a confusing
	// "no workflow named ./foo" lookup failure.
	if arg == "." || arg == ".." || strings.ContainsRune(arg, '/') {
		return false
	}
	// Existing directory relative to the invocation directory.
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		return false
	}
	// Existing directory relative to the project root, mirroring SetExecutionContext.
	if root := resolveProjectRoot(projectRootFlag); root != "" {
		if info, err := os.Stat(filepath.Join(root, arg)); err == nil && info.IsDir() {
			return false
		}
	}
	return true
}

// resolveProjectRoot returns the explicit --project-root when it points at an
// existing directory, otherwise the nearest ancestor containing project.yaml.
func resolveProjectRoot(projectRootFlag string) string {
	if projectRootFlag != "" {
		if resolved, err := filepath.Abs(projectRootFlag); err == nil {
			if info, err := os.Stat(resolved); err == nil && info.IsDir() {
				return resolved
			}
		}
		return ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	settingsPath, found, err := crecontext.FindProjectSettingsPath(cwd)
	if err != nil || !found {
		return ""
	}
	return filepath.Dir(settingsPath)
}

// GuardFolderlessTxnType rejects transaction types that need project settings
// (MSIG raw transactions and CLD changesets) in folder-less mode.
func GuardFolderlessTxnType(v *viper.Viper) error {
	if v.GetBool(settings.Flags.RawTxFlag.Name) || v.GetBool(settings.Flags.Changeset.Name) {
		return fmt.Errorf(
			"--%s and --%s require a project folder; run from your CRE project or pass the workflow folder path instead",
			settings.Flags.RawTxFlag.Name, settings.Flags.Changeset.Name,
		)
	}
	return nil
}

// ResolveFolderlessTarget resolves a workflow reference (name, ID, or empty for
// interactive picker) to the platform workflow row and its deployment registry,
// using only credentials and tenant context — no project settings.
func ResolveFolderlessTarget(
	ctx context.Context,
	runtimeContext *runtime.Context,
	arg string,
	opts workflowresolve.RowResolveOptions,
) (*FolderlessTarget, error) {
	if runtimeContext.Credentials == nil {
		return nil, fmt.Errorf("credentials not available — run `cre login` and retry")
	}
	if runtimeContext.TenantContext == nil {
		return nil, fmt.Errorf("user context not available — run `cre login` and retry")
	}

	gql := graphqlclient.New(runtimeContext.Credentials, runtimeContext.EnvironmentSet, runtimeContext.Logger)
	wdc := workflowdataclient.New(gql, runtimeContext.Logger)

	opts.Registries = runtimeContext.TenantContext.Registries
	row, err := workflowresolve.ResolveWorkflowRow(ctx, wdc, arg, opts)
	if err != nil {
		return nil, err
	}

	// The platform API returns private-registry owners without a 0x prefix;
	// normalize to the checksummed form the command validators expect.
	owner, err := ethkeys.FormatWorkflowOwnerAddress(row.OwnerAddress)
	if err != nil {
		return nil, fmt.Errorf("platform returned invalid owner address for workflow %q: %w", row.Name, err)
	}
	row.OwnerAddress = owner

	registry, err := RegistryFromWorkflowSource(runtimeContext, row.WorkflowSource)
	if err != nil {
		return nil, err
	}

	return &FolderlessTarget{Workflow: *row, Registry: registry}, nil
}

// RegistryFromWorkflowSource maps a platform workflow's source back to a
// ResolvedRegistry. It prefers the tenant-context registry match; on-chain
// "contract:<selector>:<address>" sources carry enough information to build a
// registry even when no tenant registry matches. Anything else is treated as
// an off-chain (private) registry source.
func RegistryFromWorkflowSource(runtimeContext *runtime.Context, source string) (settings.ResolvedRegistry, error) {
	donFamily := settings.EffectiveDonFamily(runtimeContext.EnvironmentSet, runtimeContext.TenantContext)
	explorerURL := ""
	if runtimeContext.EnvironmentSet != nil {
		explorerURL = runtimeContext.EnvironmentSet.WorkflowRegistryChainExplorerURL
	}

	if reg := workflowresolve.ResolveWorkflowRegistry(source, runtimeContext.TenantContext.Registries); reg != nil {
		regType, err := settings.ParseRegistryType(reg.Type)
		if err != nil {
			return nil, fmt.Errorf("registry %q: %w", reg.ID, err)
		}
		switch regType {
		case settings.RegistryTypeOffChain:
			return settings.NewOffChainRegistry(reg.ID, donFamily), nil
		case settings.RegistryTypeOnChain:
			if reg.Address == nil || strings.TrimSpace(*reg.Address) == "" {
				return nil, fmt.Errorf("on-chain registry %q has no address in user context", reg.ID)
			}
			if reg.ChainSelector == nil || strings.TrimSpace(*reg.ChainSelector) == "" {
				return nil, fmt.Errorf("on-chain registry %q has no chain selector in user context", reg.ID)
			}
			chainName, err := settings.ChainNameFromSelectorString(*reg.ChainSelector)
			if err != nil {
				return nil, fmt.Errorf("registry %q: %w", reg.ID, err)
			}
			return settings.NewOnChainRegistry(reg.ID, strings.TrimSpace(*reg.Address), chainName, donFamily, explorerURL), nil
		default:
			return nil, fmt.Errorf(
				"registry %q is not supported by this CLI version (unrecognised type from server); run `cre login` after upgrading",
				reg.ID,
			)
		}
	}

	// Fallback: the source itself encodes the on-chain registry location.
	if selector, addr, ok := workflowresolve.ParseContractWorkflowSource(source); ok && strings.TrimSpace(addr) != "" {
		chainName, err := settings.ChainNameFromSelectorString(selector)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve registry chain from workflow source %q: %w", source, err)
		}
		return settings.NewOnChainRegistry(source, strings.TrimSpace(addr), chainName, donFamily, explorerURL), nil
	}

	if strings.TrimSpace(source) == "" {
		source = "private"
	}
	return settings.NewOffChainRegistry(source, donFamily), nil
}

// VerifyEOAOwner ensures the CRE_ETH_PRIVATE_KEY in the environment derives the
// given owner address, so an on-chain folder-less operation fails fast instead
// of sending a transaction that the registry would reject.
func VerifyEOAOwner(v *viper.Viper, ownerAddress string) error {
	resolvedKey, err := settings.ResolveEthPrivateKeyFromEnv(v.GetString(settings.EthPrivateKeyEnvVar))
	if err != nil {
		return err
	}
	if !resolvedKey.IsSet() {
		return fmt.Errorf(
			"%s is not set. Set it in your .env file or export it in your shell environment to sign the transaction",
			settings.EthPrivateKeyEnvVar,
		)
	}

	derived, err := ethkeys.DeriveEthAddressFromPrivateKey(resolvedKey.Hex())
	if err != nil {
		return err
	}
	owner, err := ethkeys.FormatWorkflowOwnerAddress(ownerAddress)
	if err != nil {
		return err
	}
	if derived != owner {
		return fmt.Errorf(
			"workflow is owned by %s but %s derives %s; export the private key of the owner account",
			owner, settings.EthPrivateKeyEnvVar, derived,
		)
	}
	return nil
}
