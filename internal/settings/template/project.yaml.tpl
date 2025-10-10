# ==========================================================================
# CRE PROJECT SETTINGS FILE
# ==========================================================================
# Project-specific settings for CRE CLI targets.
# Each target defines cre-cli, account, and rpcs groups.
#
# Example custom target:
# my-target:
#   cre-cli:
#     don-family: "zone-a"                          # Required: Workflow DON Family
#   account:
#     workflow-owner-address: "0x123..."            # Optional: Owner wallet/MSIG address (used for --unsigned transactions)
#   rpcs:
#     - chain-name: ethereum-mainnet                # Required: Chain RPC endpoints
#       url: "https://mainnet.infura.io/v3/KEY"

# ==========================================================================
local-simulation:
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}

# ==========================================================================
staging:
  cre-cli:
    don-family: "{{ProductionTestnetDonFamily}}"
  account:
    workflow-owner-address: "{{WorkflowOwnerAddress}}"
  rpcs:
    - chain-name: {{EthSepoliaChainName}}
      url: {{EthSepoliaRpcUrl}}
    - chain-name: {{BaseSepoliaChainName}}
      url: {{BaseSepoliaRpcUrl}}
