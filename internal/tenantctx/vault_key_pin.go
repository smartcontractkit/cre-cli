package tenantctx

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/creconfig"
)

const VaultKeyPinsFile = "vault_key_pins.yaml"

// VaultKeyPinScope identifies the tenant vault trust anchor for TOFU pinning.
type VaultKeyPinScope struct {
	EnvName             string
	TenantID            string
	CapRegChainSelector uint64
	CapRegAddress       string
	VaultGatewayURL     string
}

// VaultKeyPin is persisted locally after a successful on-chain vault key match.
type VaultKeyPin struct {
	TenantID             string `yaml:"tenant_id"`
	CapRegChainSelector  uint64 `yaml:"cap_reg_chain_selector"`
	CapRegAddress        string `yaml:"cap_reg_address"`
	VaultGatewayURL      string `yaml:"vault_gateway_url"`
	PublicKeyFingerprint string `yaml:"public_key_fingerprint"`
}

// VaultPublicKeyFingerprint returns the SHA-256 hex digest of the vault master public key bytes.
func VaultPublicKeyFingerprint(publicKeyHex string) (string, error) {
	hexKey := strings.TrimPrefix(strings.TrimSpace(publicKeyHex), "0x")
	if hexKey == "" {
		return "", fmt.Errorf("vault public key is empty")
	}
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid vault public key hex: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// FingerprintsMatch compares two vault public key fingerprints in constant time.
func FingerprintsMatch(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(a)), []byte(strings.ToLower(b))) == 1
}

// LoadVaultKeyPin reads the pinned fingerprint for scope when the stored metadata matches.
func LoadVaultKeyPin(scope VaultKeyPinScope) (fingerprint string, ok bool, err error) {
	path, err := creconfig.FilePath(VaultKeyPinsFile)
	if err != nil {
		return "", false, err
	}
	return loadVaultKeyPinFromPath(path, scope)
}

func loadVaultKeyPinFromPath(path string, scope VaultKeyPinScope) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s: %w", VaultKeyPinsFile, err)
	}

	var pins map[string]*VaultKeyPin
	if err := yaml.Unmarshal(data, &pins); err != nil {
		return "", false, fmt.Errorf("parse %s: %w", VaultKeyPinsFile, err)
	}

	pin := pins[strings.ToUpper(strings.TrimSpace(scope.EnvName))]
	if pin == nil || pin.PublicKeyFingerprint == "" {
		return "", false, nil
	}
	if !pinMatchesScope(pin, scope) {
		return "", false, nil
	}
	return pin.PublicKeyFingerprint, true, nil
}

// SaveVaultKeyPin persists the fingerprint for scope, replacing any prior pin for the environment.
func SaveVaultKeyPin(scope VaultKeyPinScope, fingerprint string) error {
	path, err := creconfig.FilePath(VaultKeyPinsFile)
	if err != nil {
		return err
	}
	return saveVaultKeyPinToPath(path, scope, fingerprint)
}

func saveVaultKeyPinToPath(path string, scope VaultKeyPinScope, fingerprint string) error {
	if strings.TrimSpace(fingerprint) == "" {
		return fmt.Errorf("vault public key fingerprint is empty")
	}

	pins := map[string]*VaultKeyPin{}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &pins); err != nil {
			return fmt.Errorf("parse %s: %w", VaultKeyPinsFile, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", VaultKeyPinsFile, err)
	}

	envName := strings.ToUpper(strings.TrimSpace(scope.EnvName))
	pins[envName] = &VaultKeyPin{
		TenantID:             scope.TenantID,
		CapRegChainSelector:  scope.CapRegChainSelector,
		CapRegAddress:        strings.TrimSpace(scope.CapRegAddress),
		VaultGatewayURL:      strings.TrimSpace(scope.VaultGatewayURL),
		PublicKeyFingerprint: strings.ToLower(strings.TrimSpace(fingerprint)),
	}

	out, err := yaml.Marshal(pins)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", VaultKeyPinsFile, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func pinMatchesScope(pin *VaultKeyPin, scope VaultKeyPinScope) bool {
	if pin == nil {
		return false
	}
	if pin.TenantID != scope.TenantID {
		return false
	}
	if pin.CapRegChainSelector != scope.CapRegChainSelector {
		return false
	}
	if !strings.EqualFold(pin.CapRegAddress, scope.CapRegAddress) {
		return false
	}
	return strings.TrimSpace(pin.VaultGatewayURL) == strings.TrimSpace(scope.VaultGatewayURL)
}
