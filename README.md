<div style="text-align:center" align="center">
    <a href="https://chain.link" target="_blank">
        <img src="https://raw.githubusercontent.com/smartcontractkit/chainlink/develop/docs/logo-chainlink-blue.svg" width="225" alt="Chainlink logo">
    </a>

[![License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/smartcontractkit/cre-cli/blob/main/README.md)
[![CRE Documentation](https://img.shields.io/static/v1?label=CRE&message=latest&color=blue)](https://chain.link/chainlink-runtime-environment)

</div>

# Chainlink Runtime Environment (CRE) - CLI Tool

Note this README is for CRE developers only, if you are a CRE user, please ask Dev Services team for the user guide.

A command-line interface (CLI) tool for managing workflows, built with Go and Cobra. This tool allows you to compile Go workflows into WebAssembly (WASM) binaries and manage your workflow projects.

- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
   - [Sensitive Data](#sensitive-data) 
   - [Global Configuration](#global-configuration) 
   - [Secrets Template](#secrets-template) 
- [Global Flags](#global-flags)
- [Commands](#commands)
  - [Workflow Simulate](#workflow-simulate)

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/smartcontractkit/cre-cli.git
   cd cre-cli
   ```

2. Make sure you have Go installed. You can check this with:

   ```bash
   go version
   ```

3. Build the CLI tool:

   ```bash
   make build
   ```

4. (optional) Enable git pre-commit hook
    ```bash
    ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit
    ```

## Usage

You can use the CLI tool to manage workflows by running commands in the terminal. The main command is `cre`.

To view all available commands and subcommands, you can start by running the tool with `--help` flag:

```bash
./cre --help
```

To view subcommands hidden under a certain command group, select the command name and run with the tool with `--help` flag, for example:

```bash
./cre workflow --help
```

## Configuration

There are several ways to configure the CLI tool, with some configuration files only needed for running specific commands.

### Sensitive Data and `.env` file
`.env` file is used to specify sensitive data required for running most of the commands. It is **highly recommended that you don't keep the `.env` file in unencrypted format** on your disk and store it somewhere safely (e.g. in secret manager tool).
The most important environment variable to define is `CRE_ETH_PRIVATE_KEY`.

#### Using 1Password for Secret Management
* Install [1Password CLI](https://developer.1password.com/docs/cli/get-started/)
* Add variables to your 1Password Vault
* Create the `.env` file with [secret references](https://developer.1password.com/docs/cli/secret-references). Replace plaintext values with references like 
  ```
  CRE_ETH_PRIVATE_KEY=op://<vault-name>/<item-name>/[section-name/]<field-name>
  ```
* Run `cre` commands using [1Password](https://developer.1password.com/docs/cli/secrets-environment-variables/#use-environment-env-files).
  Use the op run command to provision secrets securely:
  ```shell
  op run --env-file=".env" -- cre workflow deploy myWorkflow
  ```
  _Note: `op run` doesn't support `~` inside env file path. Use only absolute or relative paths for the env file (e.g. `--env-file="/Users/username/.chainlink/cli.env"` or `--env-file="../.chainlink/cli.env"`)._

#### Exporting
To prevent any data leaks, you can also use `export` command, e.g. `export MY_ENV_VAR=mySecret`. For better security, use a space before the `export` command to prevent the command from being saved to your terminal history.

### Global Configuration
`project.yaml` file keeps CLI tool settings in one place. Once your project has been initiated using `cre init`, you will need to add a valid RPC to your `project.yaml`.

Please find more information in the project.yaml file that is created by the `cre init` command.

### Secrets Template
If you are planning on using a workflow that has a dependency on sensitive data, then it's recommended to encrypt those secrets. In such cases, a secrets template file secrets.yaml that is created by the `cre init` can be used as a starting point. Secrets template is required for the `secrets encrypt` command.

## Global Flags

All of these flags are optional, but available for each command and at each level:
- **`-h`** / **`--help`**: Prints help message.
- **`-v`** / **`--verbose`**: Enables DEBUG mode and prints more content.
- **`-R`** / **`--project-root`**: Path to project root directory.
- **`-e`** / **`--env`**: Path to .env file which contains sensitive data needed for running specific commands.

## Commands

For a list of all commands and their descriptions, please refer to the [docs](docs) folder.

### Workflow Simulate

To simulate a workflow, you can use the `cre workflow simulate` command. This command allows you to run a workflow locally without deploying it.

```bash
cre workflow simulate <path-to-workflow> --target=local-simulation
```


## Legal Notice
By using the CRE CLI tool, you agree to the Terms of Service (https://chain.link/terms) and Privacy Policy (https://chain.link/privacy-policy).
