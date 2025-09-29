# ==========================================================================
# CRE WORKFLOW SETTINGS FILE
# ==========================================================================
# This file defines environment-specific workflow settings used by the CRE CLI.
#
# Each top-level key is a target (e.g., `production`, `production-testnet`, etc.).
# You can also define your own custom targets, such as `my-target`, and
# point the CLI to it via an environment variable.
#
# Note: If any setting in this file conflicts with a setting in the CRE Project Settings File,
# the value defined here in the workflow settings file will take precedence.
#
# Below is an example `my-target`:
#
# my-target:
#   user-workflow:   
#     # Required: The name of the workflow to register with the Workflow Registry contract.
#     workflow-name: "MyExampleWorkflow"

# ==========================================================================
local-simulation:
  user-workflow:
    workflow-name: "{{WorkflowName}}"
  workflow-artifacts:
    workflow-path: "{{WorkflowPath}}"
    config-path: "./config.json"
    secrets-path: "./secrets.yaml"

# ==========================================================================
production-testnet:
  user-workflow:
    workflow-name: "{{WorkflowName}}"
  workflow-artifacts:
    workflow-path: "{{WorkflowPath}}"
    config-path: "./config.json"
    secrets-path: "./secrets.yaml"
    