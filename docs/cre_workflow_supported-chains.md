## cre workflow supported-chains

List chains and mock forwarder addresses for your tenant

### Synopsis

Lists chain selectors and mock Keystone forwarder contract addresses returned by the platform for the current tenant (from cre login / CRE_API_KEY). Chains are those enabled for your tenant.

```
cre workflow supported-chains [optional flags]
```

### Examples

```
cre workflow supported-chains
  cre workflow supported-chains --output json
```

### Options

```
  -h, --help            help for supported-chains
      --output string   Output format: "json" prints a JSON array to stdout
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
      --non-interactive       Fail instead of prompting; requires all inputs via flags
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

