# Contract deployment configuration
# Run 'cre contract deploy' to deploy contracts listed here
# Deployed addresses will be saved to deployed_contracts.yaml

# Note: Contract deployment currently requires Go bindings.
# For TypeScript workflows, you may need to deploy contracts separately
# or use Go bindings generated with 'cre generate-bindings evm'.

# Target chain for deployment (must match a chain in project.yaml rpcs)
chain: ethereum-testnet-sepolia

# List of contracts to deploy
contracts: []
  # Example: If you generate Go bindings for your contracts, add them here:
  # - name: ReserveManager
  #   package: reserve_manager
  #   deploy: true
  #   constructor: []

