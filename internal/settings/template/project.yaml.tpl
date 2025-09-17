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
#   dev-platform:
#     # Required: Workflow DON ID used for registering and operating workflows.
#     don-family: "small"
#   logging:
#     # Optional: Path to the seth configuration file (TOML). Used for logging configuration.
#     seth-config-path: "/path/to/seth-config.toml"
#   rpcs:
#     # Required: Map each used chain selector to a corresponding RPC URL (HTTPS)
#     - chain-selector: 16015286601757825753
#       url: "https://sepolia.infura.io/v3/YOUR_API_KEY"

# ==========================================================================
local-simulation:
  rpcs:
    - chain-selector: {{EthSepoliaChainSelector}}
      url: {{EthSepoliaRpcUrl}}

# ==========================================================================
production-testnet:
  dev-platform:
    don-family: "{{ProductionTestnetDonFamily}}"
  logging:
    seth-config-path: {{SethConfigPath}}
  rpcs:
    - chain-selector: {{EthSepoliaChainSelector}} # Eth-Sepolia
      url: {{EthSepoliaRpcUrl}}
    - chain-selector: {{BaseSepoliaChainSelector}} # Base-Sepolia
      url: {{BaseSepoliaRpcUrl}}
