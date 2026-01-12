# Typescript Confidential HTTP Example

This template provides a Typescript Confidential HTTP workflow example. It shows how to set a secret header and send it via the ConfidentialHTTP capability.

Steps to run the example

## 1. Update .env file

You'll need to add a secret value to the .env file for requests to read. This is the value that will be set as a header when sending requests via the ConfidentialHTTP capability.

```
SECRET_HEADER_VALUE=abcd1234
```

Note: Make sure your `workflow.yaml` file is pointing to the config.json, example:

```yaml
staging-settings:
  user-workflow:
    workflow-name: "conf-http"
  workflow-artifacts:
    workflow-path: "./main.ts"
    config-path: "./config.json"
```

## 2. Install dependencies

If `bun` is not already installed, see https://bun.com/docs/installation for installing in your environment.

```bash
cd <workflow-name> && bun install
```

Example: For a workflow directory named `conf-http` the command would be:

```bash
cd conf-http && bun install
```

## 3. Simulate the workflow

Run the command from <b>project root directory</b>

```bash
cre workflow simulate <path-to-workflow-directory> --target=staging-settings
```

Example: For workflow named `conf-http` the command would be:

```bash
cre workflow simulate ./conf-http --target=staging-settings
```
