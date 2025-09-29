# ==========================================================================
# CRE PROJECT SETTINGS FILE
# ==========================================================================
# This file defines environment-specific targets used by the CRE CLI.
# Each top-level key is a target (e.g., `production`, `production-testnet`, etc.).
#
# You can define your own custom target names, such as `my-target`, and point
# the CLI to it via an environment variable.
#
# Below is an example `my-target`:
#
# my-target:
#   cre-cli:
#     # Required: Workflow DON ID used for registering and operating workflows.
#     don-family: "small"
#   account:
#     # Optional: The address of the workflow owner (wallet or MSIG contract).
#     # Used to establish ownership for encrypting the workflow's secrets.
#     # If omitted, defaults to an empty string.
#     workflow-owner-address: "0x1234567890abcdef1234567890abcdef12345678"
#   logging:
#     # Optional: Path to the seth configuration file (TOML). Used for logging configuration.
#     seth-config-path: "/path/to/seth-config.toml"
#   rpcs:
#     # Required: Map each used chain selector to a corresponding RPC URL (HTTPS)
#     - chain-name: ethereum-mainnet
#       url: "https://sepolia.infura.io/v3/YOUR_API_KEY"

# ==========================================================================
local-simulation:
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}

# ==========================================================================
production-testnet:
  cre-cli:
    don-family: "{{ProductionTestnetDonFamily}}"
  account:
    workflow-owner-address: "{{WorkflowOwnerAddress}}"
  logging:
    seth-config-path: {{SethConfigPath}}
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}
    - chain-name: {{BaseSepoliaChainName}}
      url: {{BaseSepoliaRpcUrl}}
