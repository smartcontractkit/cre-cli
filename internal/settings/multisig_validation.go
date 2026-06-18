package settings

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// IsMultisigMode reports whether multisig transaction mode is active via --unsigned or --changeset.
func IsMultisigMode(v *viper.Viper) bool {
	return v.GetBool(Flags.RawTxFlag.Name) ||
		v.GetBool(Flags.Changeset.Name)
}

// ValidateMultisigCompatibility rejects incompatible multisig, private registry, and browser secrets auth combinations.
// resolvedRegistry may be nil during initial settings load; the private-registry check is skipped until resolved.
func ValidateMultisigCompatibility(v *viper.Viper, cmd *cobra.Command, resolvedRegistry ResolvedRegistry) error {
	if !IsMultisigMode(v) {
		return nil
	}
	if f := cmd.Flags().Lookup("secrets-auth"); f != nil && f.Value.String() == "browser" {
		return fmt.Errorf("browser secrets auth cannot be combined with multisig secrets operations; remove --unsigned/--changeset or use --secrets-auth=onchain")
	}
	if resolvedRegistry != nil && resolvedRegistry.Type() == RegistryTypeOffChain {
		return fmt.Errorf("multisig operations (--unsigned or --changeset) are not supported with private registry; remove the flag or use an on-chain deployment-registry")
	}
	return nil
}
