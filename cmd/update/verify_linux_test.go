//go:build linux

//nolint:staticcheck // SA1019: OpenPGP required to verify KMS GPG release signatures.
package update

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"

	"github.com/smartcontractkit/cre-cli/install"
)

func TestVerifyGPGSignature_validSignature(t *testing.T) {
	entity, err := openpgp.NewEntity("CRE", "Linux GPG Signing Key", "cre@smartcontract.com", nil)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cre")
	content := []byte("test-binary-content")
	require.NoError(t, os.WriteFile(binPath, content, 0600))

	sigPath := filepath.Join(tmpDir, "cre.sig")
	require.NoError(t, writeArmoredDetachedSignature(sigPath, entity, content))

	pubKey, err := exportPublicKey(entity)
	require.NoError(t, err)

	require.NoError(t, verifyGPGSignature(pubKey, binPath, sigPath))
}

func TestVerifyGPGSignature_tamperedBinary(t *testing.T) {
	entity, err := openpgp.NewEntity("CRE", "Linux GPG Signing Key", "cre@smartcontract.com", nil)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cre")
	require.NoError(t, os.WriteFile(binPath, []byte("original"), 0600))

	sigPath := filepath.Join(tmpDir, "cre.sig")
	require.NoError(t, writeArmoredDetachedSignature(sigPath, entity, []byte("original")))

	pubKey, err := exportPublicKey(entity)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(binPath, []byte("tampered"), 0600))
	err = verifyGPGSignature(pubKey, binPath, sigPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GPG signature invalid")
}

func TestVerifyGPGSignature_unexpectedSigner(t *testing.T) {
	entity, err := openpgp.NewEntity("Other", "Signer", "other@example.com", nil)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cre")
	content := []byte("test-binary-content")
	require.NoError(t, os.WriteFile(binPath, content, 0600))

	sigPath := filepath.Join(tmpDir, "cre.sig")
	require.NoError(t, writeArmoredDetachedSignature(sigPath, entity, content))

	pubKey, err := exportPublicKey(entity)
	require.NoError(t, err)

	err = verifyGPGSignature(pubKey, binPath, sigPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected signer identity")
}

func TestVerifyGPGSignature_embeddedReleaseKeyParses(t *testing.T) {
	require.NotEmpty(t, install.ReleasePublicKey)
	_, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(install.ReleasePublicKey))
	require.NoError(t, err)
}

func writeArmoredDetachedSignature(path string, entity *openpgp.Entity, content []byte) error {
	var sigBuf bytes.Buffer
	armorWriter, err := armor.Encode(&sigBuf, openpgp.SignatureType, nil)
	if err != nil {
		return err
	}
	if err := openpgp.DetachSign(armorWriter, entity, bytes.NewReader(content), nil); err != nil {
		return err
	}
	if err := armorWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, sigBuf.Bytes(), 0600)
}

func exportPublicKey(entity *openpgp.Entity) ([]byte, error) {
	var buf bytes.Buffer
	armorWriter, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return nil, err
	}
	if err := entity.Serialize(armorWriter); err != nil {
		return nil, err
	}
	if err := armorWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestValidateSignerIdentity(t *testing.T) {
	validEntity, err := openpgp.NewEntity("CRE", "Linux GPG Signing Key", "cre@smartcontract.com", nil)
	require.NoError(t, err)
	require.NoError(t, validateSignerIdentity(validEntity))

	invalidEntity, err := openpgp.NewEntity("Evil", "Signer", "evil@example.com", nil)
	require.NoError(t, err)
	require.Error(t, validateSignerIdentity(invalidEntity))
}
