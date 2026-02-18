package decrypt_output

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// New creates and returns the 'secrets decrypt-output' cobra command.
func New(_ *runtime.Context) *cobra.Command {
	var (
		passphrase string
		input      string
		encoding   string
	)

	cmd := &cobra.Command{
		Use:   "decrypt-output",
		Short: "Decrypts an AES-GCM encrypted response body using a passphrase.",
		Long: `Derives the AES-256 key from the given passphrase (same HKDF-SHA256 derivation
as store-encryption-key) and decrypts the provided ciphertext.

This is a purely local operation; no VaultDON interaction required.`,
		Example: `  # Decrypt base64-encoded ciphertext from a file
  cre secrets decrypt-output --passphrase "my-secret" --input encrypted.b64

  # Decrypt from stdin (pipe)
  echo "<base64>" | cre secrets decrypt-output --passphrase "my-secret" --input -

  # Decrypt hex-encoded ciphertext
  cre secrets decrypt-output --passphrase "my-secret" --input encrypted.hex --encoding hex

  # Decrypt raw binary ciphertext
  cre secrets decrypt-output --passphrase "my-secret" --input encrypted.bin --encoding raw`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if passphrase == "" {
				return fmt.Errorf("--passphrase is required and must not be empty")
			}
			if input == "" {
				return fmt.Errorf("--input is required")
			}

			raw, err := readInput(input)
			if err != nil {
				return err
			}

			ciphertext, err := decodeCiphertext(raw, encoding)
			if err != nil {
				return err
			}

			key, err := common.DeriveEncryptionKey(passphrase)
			if err != nil {
				return fmt.Errorf("failed to derive encryption key: %w", err)
			}

			plaintext, err := common.AESGCMDecrypt(ciphertext, key)
			if err != nil {
				return fmt.Errorf("decryption failed: %w", err)
			}

			_, err = os.Stdout.Write(plaintext)
			return err
		},
	}

	cmd.Flags().StringVar(&passphrase, "passphrase", "", "Passphrase used to derive the AES-256 decryption key (required)")
	cmd.Flags().StringVarP(&input, "input", "i", "", "File path containing ciphertext, or '-' for stdin (required)")
	cmd.Flags().StringVar(&encoding, "encoding", "base64", "Encoding of the input ciphertext: base64, hex, or raw")
	_ = cmd.MarkFlagRequired("passphrase")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	return data, nil
}

func decodeCiphertext(raw []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(string(raw))
			if err != nil {
				return nil, fmt.Errorf("base64 decode failed: %w", err)
			}
		}
		return decoded, nil
	case "hex":
		decoded, err := hex.DecodeString(string(raw))
		if err != nil {
			return nil, fmt.Errorf("hex decode failed: %w", err)
		}
		return decoded, nil
	case "raw":
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q: use base64, hex, or raw", encoding)
	}
}
