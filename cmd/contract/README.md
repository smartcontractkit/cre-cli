# Contract Deployment

The `cre contract deploy` command compiles and deploys smart contracts to the blockchain.

## Prerequisites

### Install Foundry

Foundry is required to compile Solidity contracts. Install it by running:

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

## Quick Start

### Step 1: Initialize a New Project

```bash
cre init
```

When prompted, select:
- **Language**: `Golang`
- **Template**: `Custom data feed: Updating on-chain data periodically using offchain API data`
- **RPC URL**: Enter your Sepolia RPC URL (e.g., `https://ethereum-sepolia-rpc.publicnode.com`)

Example:
```
Project name? [my-project]: my-project
✔ Golang
✔ Custom data feed: Updating on-chain data periodically using offchain API data
Sepolia RPC URL? [https://ethereum-sepolia-rpc.publicnode.com]: https://ethereum-sepolia-rpc.publicnode.com
Workflow name? [my-workflow]: my-workflow
```

### Step 2: Navigate to Your Project

```bash
cd my-project
```

### Step 3: Configure Your Private Key

Edit the `.env` file and add your private key:

```bash
CRE_ETH_PRIVATE_KEY=0x...your_private_key...
```

> ⚠️ **Important**: Never commit your private key to version control. The `.env` file should be in `.gitignore`.

### Step 4: Deploy Contracts

```bash
cre contract deploy
```

The command will:
1. Compile all Solidity contracts using Foundry
2. Display the contracts to be deployed
3. Ask for confirmation
4. Deploy each contract and display the address

Example output:
```
Compiling contracts with Foundry...
Compiler run successful!

Contract Deployment
===================
Project Root:    /path/to/my-project
Target Chain:    ethereum-testnet-sepolia
Config File:     /path/to/my-project/contracts/contracts.yaml

Contracts:
  - BalanceReader (balance_reader): deploy
  - MessageEmitter (message_emitter): deploy
  - ReserveManager (reserve_manager): deploy
  - IERC20 (ierc20): skip

Deploying BalanceReader...
  Address: 0xf95DF418d791e8da0D12C6E88Bc4443a056A9E22
  Tx Hash: 0x62815513c355be832cab16bb9840d1c26de0e074efe65441591a09b28c7e68a2

Deploying MessageEmitter...
  Address: 0x63Cb753C77908cbD2Cc9A4B37B0D6DC7F5fF00a1
  Tx Hash: 0x741bb347f74aaa85ae9c97bcb670de4cf2b6f638a0a14a837ff0508e6f3c0c94

Deploying ReserveManager...
  Address: 0x8cFc0495AaAAF2fa6BC39eaaA5952d5027e79C88
  Tx Hash: 0xb47ffb1f8d80e17852644cd0ea38d4dfed8a287a81753c6fcf13ff7867f75683

[OK] Contracts deployed successfully
Deployed addresses saved to: /path/to/my-project/contracts/deployed_contracts.yaml
```

## Command Options

```bash
cre contract deploy [flags]
```

| Flag | Description |
|------|-------------|
| `--chain` | Override the target chain from contracts.yaml |
| `--dry-run` | Validate configuration without deploying |
| `--yes` | Skip confirmation prompt |
| `-v, --verbose` | Show detailed logs |

### Examples

```bash
# Deploy with confirmation prompt
cre contract deploy

# Deploy without confirmation
cre contract deploy --yes

# Validate without deploying
cre contract deploy --dry-run

# Deploy to a different chain
cre contract deploy --chain ethereum-testnet-sepolia
```

## Configuration

### contracts.yaml

Located at `contracts/contracts.yaml`, this file defines which contracts to deploy:

```yaml
chain: ethereum-testnet-sepolia
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

  - name: IERC20
    package: ierc20
    deploy: false  # Skip deployment (interface only)
```

### deployed_contracts.yaml

After deployment, contract addresses are saved to `contracts/deployed_contracts.yaml`:

```yaml
chain_id: 16015286601757825753
chain_name: ethereum-testnet-sepolia
timestamp: "2026-01-06T22:00:37Z"
contracts:
    BalanceReader:
        address: 0xf95DF418d791e8da0D12C6E88Bc4443a056A9E22
        tx_hash: 0x62815513c355be832cab16bb9840d1c26de0e074efe65441591a09b28c7e68a2
    MessageEmitter:
        address: 0x63Cb753C77908cbD2Cc9A4B37B0D6DC7F5fF00a1
        tx_hash: 0x741bb347f74aaa85ae9c97bcb670de4cf2b6f638a0a14a837ff0508e6f3c0c94
    ReserveManager:
        address: 0x8cFc0495AaAAF2fa6BC39eaaA5952d5027e79C88
        tx_hash: 0xb47ffb1f8d80e17852644cd0ea38d4dfed8a287a81753c6fcf13ff7867f75683
```

## Using Deployed Addresses in Workflows

After deployment, you can reference contract addresses in your workflow configuration using placeholders:

```json
{
  "contract_address": "{{contracts.MessageEmitter.address}}"
}
```

These placeholders are automatically replaced with actual addresses when you run `cre workflow deploy`.

## Troubleshooting

### "forge is required but not installed"

Install Foundry:
```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

### "failed to parse private key"

Ensure your `.env` file contains a valid private key:
```bash
CRE_ETH_PRIVATE_KEY=0x... # Must be a valid hex string
```

### "no RPC URL configured for chain"

Check that your `project.yaml` has the correct RPC configuration:
```yaml
rpcs:
  - chain_name: ethereum-testnet-sepolia
    url: https://ethereum-sepolia-rpc.publicnode.com
```

### Compilation errors

If contracts fail to compile, check:
1. Solidity version compatibility in your `.sol` files
2. All imported files exist in `contracts/evm/src/`
3. Import paths are correct (use relative paths like `./keystone/IReceiver.sol`)

