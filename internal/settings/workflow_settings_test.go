package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSettings_ExperimentalChains(t *testing.T) {
	t.Run("passes validation for experimental chain with chain-selector and forwarder", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "experimental-chain",
					Url:           "https://rpc.example.com",
					ChainSelector: 5299555114858065850,
					Forwarder:     "0x76c9cf548b4179F8901cda1f8623568b58215E62",
				},
			},
		}

		err := validateSettings(config)
		require.NoError(t, err)
	})

	t.Run("fails validation for experimental chain with chain-selector but no forwarder", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "experimental-chain",
					Url:           "https://rpc.example.com",
					ChainSelector: 5299555114858065850,
					Forwarder:     "", // missing forwarder
				},
			},
		}

		err := validateSettings(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires forwarder")
		assert.Contains(t, err.Error(), "5299555114858065850")
	})

	t.Run("fails validation for experimental chain with whitespace-only forwarder", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "experimental-chain",
					Url:           "https://rpc.example.com",
					ChainSelector: 5299555114858065850,
					Forwarder:     "   ", // whitespace-only forwarder
				},
			},
		}

		err := validateSettings(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires forwarder")
	})

	t.Run("skips chain-name validation for experimental chains", func(t *testing.T) {
		// An experimental chain with an invalid chain name should still pass validation
		// because chain-name validation is skipped for experimental chains
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "not-a-real-chain-name", // invalid chain name
					Url:           "https://rpc.example.com",
					ChainSelector: 5299555114858065850, // marks as experimental
					Forwarder:     "0x76c9cf548b4179F8901cda1f8623568b58215E62",
				},
			},
		}

		err := validateSettings(config)
		require.NoError(t, err) // should pass because chain-name validation is skipped
	})

	t.Run("validates chain-name for non-experimental chains", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "invalid-chain-name", // invalid chain name
					Url:           "https://rpc.example.com",
					ChainSelector: 0, // not experimental
				},
			},
		}

		err := validateSettings(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chain name")
	})

	t.Run("validates URL for experimental chains", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "experimental-chain",
					Url:           "not-a-valid-url",
					ChainSelector: 5299555114858065850,
					Forwarder:     "0x76c9cf548b4179F8901cda1f8623568b58215E62",
				},
			},
		}

		err := validateSettings(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid rpc url")
	})

	t.Run("validates URL for non-experimental chains", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName:     "ethereum-testnet-sepolia",
					Url:           "ftp://invalid-scheme.com", // invalid scheme
					ChainSelector: 0,
				},
			},
		}

		err := validateSettings(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scheme")
	})

	t.Run("handles mixed experimental and non-experimental chains", func(t *testing.T) {
		config := &WorkflowSettings{
			RPCs: []RpcEndpoint{
				{
					ChainName: "ethereum-testnet-sepolia",
					Url:       "https://sepolia.rpc.org",
					// ChainSelector is 0 (non-experimental)
				},
				{
					ChainName:     "worldchain-sepolia",
					Url:           "https://worldchain.rpc.org",
					ChainSelector: 5299555114858065850,
					Forwarder:     "0x76c9cf548b4179F8901cda1f8623568b58215E62",
				},
			},
		}

		err := validateSettings(config)
		require.NoError(t, err)
	})
}
