# Contract deployment configuration
# Run 'cre contract deploy' to deploy contracts listed here
# Deployed addresses will be saved to deployed_contracts.yaml

# Target chain for deployment (must match a chain in project.yaml rpcs)
chain: ethereum-testnet-sepolia

# List of contracts to deploy
# Each contract must have Go bindings in contracts/evm/src/generated/{package}/
# Generate bindings with: cre generate-bindings evm
contracts: []
  # Example contract configuration:
  # - name: MyContract           # Contract name (must match binding name)
  #   package: my_contract       # Go package name from generated bindings
  #   deploy: true               # Set to false to skip deployment
  #   constructor:               # Constructor arguments (if any)
  #     - type: address
  #       value: "0x..."
  #     - type: uint256
  #       value: "1000000"

