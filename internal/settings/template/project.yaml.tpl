# ==========================================================================
# CRE PROJECT SETTINGS FILE
# ==========================================================================
# Project-specific settings for CRE CLI targets.
# Each target defines cre-cli, account, and rpcs groups.
#
# Example custom target:
# my-target:
#   account:
#     workflow-owner-address: "0x123..."            # Optional: Owner wallet/MSIG address (used for --unsigned transactions)
#   rpcs:
#     - chain-name: ethereum-testnet-sepolia        # Required if your workflow interacts with this chain
#       url: "<select your own rpc url>"
#     - chain-name: ethereum-mainnet                # Required if your workflow interacts with this chain
#       url: "<select your own rpc url>"
#
# Experimental chains (automatically used by the simulator when present):
# Use this for chains not yet in official chain-selectors (e.g., hackathons, new chain integrations).
# Add chain-selector and forwarder to an rpcs entry to mark it as experimental.
# In your workflow, reference the chain as evm:ChainSelector:<chain-selector>@1.0.0
#
#     - chain-name: my-experimental-chain           # Optional label for the chain
#       chain-selector: 5299555114858065850         # Chain selector (required for experimental)
#       url: "https://rpc.example.com"              # RPC endpoint URL
#       forwarder: "0x..."                          # Forwarder contract address (required when chain-selector is set)

# ==========================================================================
staging-settings:
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}

# ==========================================================================
production-settings:
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}
