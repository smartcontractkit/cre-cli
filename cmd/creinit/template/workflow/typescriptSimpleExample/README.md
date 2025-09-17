# Typescript Simple Workflow Example

This template provides a simple Typescript workflow example. It shows how to create a simple "Hello World" workflow using Typescript.

Steps to run the example

## 1. Update .env file

You need to add a private key to env file. This is specifically required if you want to simulate chain writes. For that to work the key should be valid and funded.
If your workflow does not do any chain write then you can just put any dummy key as a private key. e.g.
```
CRE_ETH_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000001
```

## 2. Install dependencies
```
cd workflowName && bun install
```

## 3. Simulate the workflow
Run the command from <b>workflow root directory</b> (Run `cd workflowName` if you are in project root directory)

```bash
cre workflow simulate --target local-simulation --config config.json main.ts
```
