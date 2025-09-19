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
#     # Optional: The address of the workflow owner (wallet or MSIG contract).
#     # Used to establish ownership for encrypting the workflow's secrets.
#     # If omitted, defaults to an empty string.
#     workflow-owner-address: "0x1234567890abcdef1234567890abcdef12345678"
#     
#     # Required: The name of the workflow to register with the Workflow Registry contract.
#     workflow-name: "MyExampleWorkflow"

# ==========================================================================
local-simulation:
  user-workflow:
    workflow-owner-address: "{{WorkflowOwnerAddress}}"
    workflow-name: "{{WorkflowName}}"

# ==========================================================================
production-testnet:
  user-workflow:
    workflow-owner-address: "{{WorkflowOwnerAddress}}"
    workflow-name: "{{WorkflowName}}"
