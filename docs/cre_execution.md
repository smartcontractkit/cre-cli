## cre execution

Query workflow execution history

### Synopsis

The execution command provides visibility into workflow executions, node events, and logs.

```
cre execution [optional flags]
```

### Options

```
  -h, --help   help for execution
```

### Options inherited from parent commands

```
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool
* [cre execution events](cre_execution_events.md)	 - Show the node/capability event timeline for an execution
* [cre execution list](cre_execution_list.md)	 - List recent executions for a workflow
* [cre execution logs](cre_execution_logs.md)	 - Show logs emitted during a workflow execution
* [cre execution status](cre_execution_status.md)	 - Show detailed status of a single execution

