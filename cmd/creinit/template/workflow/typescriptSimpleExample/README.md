# Typescript Simple Workflow Example

This template provides a simple Typescript workflow example. It shows how to create a simple "Hello World" workflow using Typescript.

Steps to run the example

## 1. Update .env file

You need to add a private key to env file. This is specifically required if you want to simulate chain writes. For that to work the key should be valid and funded.
If your workflow does not do any chain write then you can just put any dummy key as a private key. e.g.

```
CRE_ETH_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000001
```

Note: Make sure your `project.yaml` file is pointing to the config.json, example:

```yaml
local-simulation:
  user-workflow:
    workflow-name: "hello-world"
  workflow-artifacts:
    workflow-path: "./main.ts"
    config-path: "./config.json"
```

## 2. Install dependencies

```bash
cd <workflow-name> && bun install
```

Example: For workflow named `hello-world` the command would be:

```bash
cd hello-world && bun install
```

## 3. Simulate the workflow

Run the command from <b>project root directory</b>

```bash
cre workflow simulate <path-to-workflow-directory> --target local-simulation
```

Example: For workflow named `hello-world` the command would be:

```bash
cre workflow simulate ./hello-world --target local-simulation
```
