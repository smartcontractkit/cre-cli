# Contract deployment configuration
# Run 'cre contract deploy' to deploy contracts listed here
# Deployed addresses will be saved to deployed_contracts.yaml

# Target chain for deployment (must match a chain in project.yaml rpcs)
chain: ethereum-testnet-sepolia

# List of contracts to deploy
# Each contract must have Go bindings in contracts/evm/src/generated/{package}/
# Generate bindings with: cre generate-bindings evm
contracts:
  - name: BalanceReader
    package: balance_reader
    deploy: true
    constructor: []

  - name: MessageEmitter
    package: message_emitter
    deploy: true
    constructor: []

  - name: ReserveManager
    package: reserve_manager
    deploy: true
    constructor: []

  # IERC20 is typically an existing token contract, not deployed
  - name: IERC20
    package: ierc20
    deploy: false

