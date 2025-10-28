# ==========================================================================
# CRE WORKFLOW SETTINGS FILE
# ==========================================================================
# Workflow-specific settings for CRE CLI targets.
# Each target defines user-workflow and workflow-artifacts groups.
# Settings here override CRE Project Settings File values.
#
# Example custom target:
# my-target:
#   user-workflow:
#     workflow-name: "MyExampleWorkflow"    # Required: Workflow Registry name
#   workflow-artifacts:
#     workflow-path: "./main.ts"            # Path to workflow entry point
#     config-path: "./config.yaml"          # Path to config file
#     secrets-path: "../secrets.yaml"       # Path to secrets file (project root by default)

# ==========================================================================
staging-settings:
  user-workflow:
    workflow-name: "{{WorkflowName}}-staging"
  workflow-artifacts:
    workflow-path: "{{WorkflowPath}}"
    config-path: "{{ConfigPathStaging}}"
    secrets-path: "{{SecretsPath}}"
    

# ==========================================================================
production-settings:
  user-workflow:
    workflow-name: "{{WorkflowName}}-production"
  workflow-artifacts:
    workflow-path: "{{WorkflowPath}}"
    config-path: "{{ConfigPathProduction}}"
    secrets-path: "{{SecretsPath}}"