//go:build linux

package update

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

//go:embed ../../install/public_key.asc
var releasePublicKey []byte

func verifyReleaseBinary(binPath, sigPath string) error {
	if sigPath == "" {
		return fmt.Errorf("missing signature file path")
	}
	return verifyGPGSignature(releasePublicKey, binPath, sigPath)
}

func verifyGPGSignature(publicKey []byte, binPath, sigPath string) error {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicKey))
	if err != nil {
		return fmt.Errorf("parse release public key: %w", err)
	}

	signed, err := os.Open(binPath) // #nosec G703 -- path from controlled temp extraction
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer signed.Close()

	sigBytes, err := os.ReadFile(sigPath) // #nosec G703 -- path from controlled temp download
	if err != nil {
		return fmt.Errorf("read signature: %w", err)
	}

	entity, err := checkDetachedSignature(keyring, signed, sigBytes)
	if err != nil {
		return fmt.Errorf("GPG signature invalid: %w", err)
	}

	if err := validateSignerIdentity(entity); err != nil {
		return err
	}
	return nil
}

func checkDetachedSignature(keyring openpgp.KeyRing, signed *os.File, sigBytes []byte) (*openpgp.Entity, error) {
	if _, err := signed.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind binary: %w", err)
	}

	sigReader := bytes.NewReader(sigBytes)
	if block, _ := armor.Decode(sigReader); block != nil {
		if _, err := signed.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("rewind binary: %w", err)
		}
		entity, err := openpgp.CheckDetachedSignature(keyring, signed, block.Body)
		if err != nil {
			return nil, err
		}
		return entity, nil
	}

	if _, err := signed.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind binary: %w", err)
	}
	entity, err := openpgp.CheckArmoredDetachedSignature(keyring, signed, bytes.NewReader(sigBytes))
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func validateSignerIdentity(entity *openpgp.Entity) error {
	if entity == nil {
		return fmt.Errorf("missing signer identity")
	}
	for _, identity := range entity.Identities {
		if strings.Contains(identity.Name, expectedSignerName) &&
			strings.EqualFold(identity.Email, expectedSignerEmail) {
			return nil
		}
	}
	return fmt.Errorf("unexpected signer identity")
}
