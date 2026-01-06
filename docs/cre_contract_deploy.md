## cre contract deploy

Deploys smart contracts to the blockchain

### Synopsis

Deploys smart contracts defined in contracts/contracts.yaml to the target blockchain.
The deployed contract addresses are stored in contracts/deployed_contracts.yaml
and can be referenced in workflow configurations using placeholders.

```
cre contract deploy [optional flags]
```

### Examples

```
  cre contract deploy
  cre contract deploy --dry-run
  cre contract deploy --chain ethereum-testnet-sepolia
```

### Options

```
      --chain string   Override the target chain from contracts.yaml
      --dry-run        Validate configuration without deploying contracts
  -h, --help           help for deploy
      --yes            If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre contract](cre_contract.md)	 - Manages smart contracts

