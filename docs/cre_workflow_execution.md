## cre workflow execution

Query workflow execution history

### Synopsis

The execution command provides visibility into workflow executions, node events, and logs.

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

* [cre workflow](cre_workflow.md)	 - Manages workflows
* [cre workflow execution events](cre_workflow_execution_events.md)	 - Show the node/capability event timeline for an execution
* [cre workflow execution list](cre_workflow_execution_list.md)	 - List recent executions for a workflow
* [cre workflow execution logs](cre_workflow_execution_logs.md)	 - Show logs emitted during a workflow execution
* [cre workflow execution status](cre_workflow_execution_status.md)	 - Show detailed status of a single execution

