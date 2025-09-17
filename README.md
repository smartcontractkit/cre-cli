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

[//]: # (   - [Run Tests]&#40;#run-tests&#41;)

[//]: # (   - [Compile a Workflow]&#40;#compile-a-workflow&#41;)

[//]: # (   - [Generate Encrypted Secrets]&#40;#generate-encrypted-secrets&#41;)

[//]: # (   - [Upload Gists]&#40;#upload-gists&#41;)

[//]: # (   - [Deploy Workflow]&#40;#deploy-workflow&#41;)

[//]: # (   - [How to get a valid Github API token]&#40;#how-to-get-a-valid-github-api-token&#41;)

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/smartcontractkit/dev-platform.git
   cd dev-platform
   ```

2. Make sure you have Go installed. You can check this with:

   ```bash
   go version
   ```

3. Build the CLI tool:

   ```bash
   make build
   ```
   or
   ```bash
   go build -ldflags "-w" -o cre
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
The most important environment variables to define are `CRE_ETH_PRIVATE_KEY` and `CRE_GITHUB_API_TOKEN`.

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
  op run --env-file=".env" -- cre workflow list
  ```
  _Note: `op run` doesn't support `~` inside env file path. Use only absolute or relative paths for the env file (e.g. `--env-file="/Users/username/.chainlink/cli.env"` or `--env-file="../.chainlink/cli.env"`)._

#### Exporting
To prevent any data leaks, you can also use `export` command, e.g. `export MY_ENV_VAR=mySecret`. For better security, use a space before the `export` command to prevent the command from being saved to your terminal history.

### Global Configuration
`project.yaml` file keeps CLI tool settings in one place. It is used to reconfigure the tool in cases such as using a different Capabilities or Workflow Registry, using a different Workflow DON, or a different workflow owner address.

Please find more information in the project.yaml file that is created by the `cre init` command.

### Secrets Template
If you are planning on using a workflow that has a dependency on sensitive data, then it's recommended to encrypt those secrets. In such cases, a secrets template file secrets.yaml that is created by the `cre init` can be used as a starting point. Secrets template is required for the `secrets encrypt` command.

## Global Flags

All of these flags are optional, but available for each command and at each level:
- **`-h`** / **`--help`**: Prints help message.
- **`-v`** / **`--verbose`**: Enables DEBUG mode and prints more content.
- **`-S`** / **`--workflow-settings-file`**: Path to CLI tool settings file, contains configuration needed for running specific commands.
- **`-e`** / **`--env`**: Path to .env file which contains sensitive data needed for running specific commands.

## Commands

For a list of all commands and their descriptions, please refer to the [docs](docs) folder.

### Workflow Simulate

To simulate a workflow, you can use the `cre workflow simulate` command. This command allows you to run a workflow locally without deploying it.

```bash
cre workflow simulate --target local-simulation --config <path-to-config.json> <path-to-workflow-file>
```

[//]: # (### Run Tests)

[//]: # ()
[//]: # (To test your Go file before compiling, you may use the command `cre test ./path/to/test`.)

[//]: # ()
[//]: # (You may also add the optional `--run` flag to only run tests with names that match the input regular expression.)

[//]: # ()
[//]: # (Example: `cre workflow test ./path/to/test --run MyTestName`)

[//]: # ()
[//]: # (### Compile a Workflow)

[//]: # ()
[//]: # (To compile a Go workflow into a WASM binary, use one of the following commands:)

[//]: # ()
[//]: # (1. **Using the built CLI tool:**)

[//]: # ()
[//]: # (   ```bash)

[//]: # (   cre compile <path-to-workflow-file>)

[//]: # (   ```)

[//]: # ()
[//]: # (   **Example:**)

[//]: # ()
[//]: # (   ```bash)

[//]: # (   cre compile ./workflows/workflowDemo.go)

[//]: # (   ```)

[//]: # ()
[//]: # (2. **Alternatively, you can run it directly with Go:**)

[//]: # ()
[//]: # (   ```bash)

[//]: # (   go run main.go workflow compile <path-to-workflow-file>)

[//]: # (   ```)

[//]: # ()
[//]: # (   **Example:**)

[//]: # ()
[//]: # (   ```bash)

[//]: # (   go run main.go workflow compile ../workflow-starter-kit/workflows/por/workflowDemo.go)

[//]: # (   ```)

[//]: # ()
[//]: # (By default, both commands will:)

[//]: # (- Compile the specified Go file into a WASM binary.)

[//]: # (- Read the config JSON file &#40;if provided via the `--config` flag followed by the path to the config file&#41;)

[//]: # (- Perform DAG verification on the WASM binary &#40;and config file if provided&#41;)

[//]: # (- Compress the binary using Brotli compression.)

[//]: # (- Create new Gists for the binary and config files &#40;use the `--no-gist` flag to disable this&#41;)

[//]: # (  - If creating a Gist, ensure the `CRE_GITHUB_API_TOKEN` environment variable is set. [See here]&#40;#how-to-get-a-valid-github-api-token&#41;.)

[//]: # (  - Use the `--env` flag to specify the path to a `.env` file. By default, the `.env` file in the current working directory will be used.)

[//]: # ()
[//]: # (This process generates a compressed WASM file:)

[//]: # (- `binary.wasm.br` &#40;the Brotli-compressed version of the binary&#41;.)

[//]: # ()
[//]: # (You can also specify a custom output path and filename for the compiled WASM binary using the `--output` flag. This allows you to control where the files are generated.)

[//]: # ()
[//]: # (```bash)

[//]: # (cre compile <path-to-workflow-file> --output=<output-path>)

[//]: # (```)

[//]: # ()
[//]: # (**Example:**)

[//]: # ()
[//]: # (```bash)

[//]: # (cre compile ./workflows/workflowDemo.go --output=./myWorkflow.wasm.br)

[//]: # (```)

[//]: # ()
[//]: # (In this example:)

[//]: # (- The compressed version will be saved as `myWorkflow.wasm.br` in the same directory.)

[//]: # ()
[//]: # (If no `--output` flag is provided, the default filenames are `binary.wasm.br`.)

[//]: # (### Generate Encrypted Secrets)

[//]: # ()
[//]: # (1. Create secrets template for desired workflow similar to [`example.secrets.config.yaml`]&#40;example.secrets.config.yaml&#41;)

[//]: # (2. Enter the secret names and corresponding environment variables into the `secretNames` field in `secrets.config.yaml`)

[//]: # (   - **DO NOT ENTER PLAINTEXT SECRETS!!!** Only environment variable names.)

[//]: # (   - The secret names are used to reference the secret within the workflow.)

[//]: # (   - Notice that each secret name can be assigned multiple environment variables. This allows for giving each node a different secret.)

[//]: # (     - If the number of environment variables for a given secret is less than the total number of nodes in the DON, the environment variable values will be assigned to nodes in round-robin fashion. You may also specify only a single secret to be used across all nodes.)

[//]: # (     - If the number of environment variables for a given secret is more than the total number of nodes in the DON, not all the environment variable values will be used.)

[//]: # (     - It is **highly recommended** to use a separate secret for each node to reduce the impact of a leaked key.)

[//]: # (3. Ensure the following environment variables are set. It is **highly recommended** to set any secret environment variables *without* placing the raw secret values in a `.env` file. A better method is using ` export` commands, ie: ` export MY_ENV_VAR=mySecret`. For further security, use a space before the ` export` to prevent the command from being saved to your terminal history.)

[//]: # (   -  `CRE_GITHUB_API_TOKEN`: This is required for creating or updating Gists. See [How to get a valid Github API token]&#40;#how-to-get-a-valid-github-api-token&#41;.)

[//]: # (4. Ensure the following settings are correctly set in your `cre.setting.yaml` file:)

[//]: # (   - `workflow_owner_address`: Address of the wallet / multisig which will own the workflow using the encrypted secrets. Can be overridden with the `--owner` flag.)

[//]: # (     - This is required to establish secrets ownership to prevent an unauthorized owner from attempting to use the secrets in their own workflow.)

[//]: # (   -  `CapabilitiesRegistry` contract information: Address and chain selector of the CapabilitiesRegistry contract which holds the public encryption keys)

[//]: # (   -  `don_id`: Default DON ID to use)

[//]: # (   - Finally, set `WorkflowRegistry` contract information, along with necessary RPC information &#40;or copy defaults from [`example.cre.settings.yaml` file]&#40;example.cre.settings.yaml&#41;&#41;)

[//]: # (5. Run `cre secrets encrypt`. Note that the following **optional** CLI flags can also be used:)

[//]: # (   - `--gist-id`: Provide a previous Gist ID to update an existing Gist)

[//]: # (   - `--env`: Path to .env file &#40;defaults to `.env` in the current working directory&#41;)

[//]: # (   - `--owner`: Overrides the `workflow_owner_address` setting)

[//]: # (   - `--secrets-config`: Path to YAML configuration file &#40;defaults to `secrets.config.yaml` in the current working directory&#41;)

[//]: # (   - `--output`: Path to output file &#40;defaults to `encrypted.secrets.json`&#41;)

[//]: # ()
[//]: # (### Upload Gists)

[//]: # ()
[//]: # (While there is built-in functionality for uploading Gists in the `compile` and `encrypt` commands, a user may want the ability to upload specific files to Gists themselves. For example, a user may want to upload only a new config file, but use an existing binary.)

[//]: # ()
[//]: # (1. Ensure the `CRE_GITHUB_API_TOKEN` environment variable is set. See [How to get a valid Github API token]&#40;#how-to-get-a-valid-github-api-token&#41;.)

[//]: # (2. If you want to create or update only one Gist, run `cre upload single fileName`. Note that the following **optional** CLI flags can also be used:)

[//]: # (   - `--gist-id`: Provide Gist IDs to update, if not provided, Gist will be created.)

[//]: # (   - `--env`: Path to .env file &#40;defaults to `.env` in the current working directory&#41;)

[//]: # (   - Execution example:)

[//]: # (   ```bash)

[//]: # (   cre upload single ./encryptedSecrets.json --gist-id ccb63813954654f1d3400223c45d5761)

[//]: # (   ```)

[//]: # (3. If you want to create or update multiple Gists, run `cre upload batch`. This command is using a **required** flag `--file` to specify files to upload. You can specify one or more of them. Note that the following **optional** CLI flags can also be used:)

[//]: # (   - `--gist-id`: Provide Gist IDs to update. You can specify one or more of them. Note that number of Gist IDs must match number of files.)

[//]: # (   - `--env`: Path to .env file &#40;defaults to `.env` in the current working directory&#41;)

[//]: # (   - Execution example &#40;first file will match the first specified Gist ID&#41;:)

[//]: # (   ```bash)

[//]: # (   cre upload batch --file ./encryptedSecrets.json --file ./config.yaml --gist-id ccb63813954654f1d3400223c45d5761 --gist-id ccb63813954654f1d3400223c45d5761)

[//]: # (   ```)

[//]: # ()
[//]: # (### Deploy Workflow)

[//]: # ()
[//]: # (Once the workflow binary has been uploaded &#40;alongside the config YAML and encrypted secrets JSON files if necessary&#41;, the workflow can now be deployed onchain.)

[//]: # ()
[//]: # (- Run the command `cre workflow deploy YOUR_WORKFLOW_NAME --binary-url https://website.com/path/to/your/binary`)

[//]: # (  - First, ensure that `workflow_owner_address` and `don_id` are properly set in the `cre.settings.yaml` file. Additionaly, set `WorkflowRegistry` and `CapabilitiesRegistry` contracts, along with necessary RPC information in the `cre.settings.yaml` file &#40;or copy defaults from [`example.cre.settings.yaml` file]&#40;example.cre.settings.yaml&#41;&#41;. Note that setting `workflow_owner_address` can also be overriden by `--owner` flag using this command.)

[//]: # (- Note these additional optional CLI flags:)

[//]: # (   - `--config-url`: URL for the uploaded configuration YAML file)

[//]: # (   - `--secrets-url`: URL of the uploaded encrypted secrets JSON file)

[//]: # (   - `--auto-start`: Disable automatically starting the workflow at registration by setting to `false` &#40;defaults to `true`&#41;)

[//]: # (   - `--env`: Path to .env file &#40;defaults to `.env` in the current working directory&#41;)

[//]: # (   - `--owner`: Overrides the `workflow_owner_address` setting)

[//]: # (   - `--output`: Path to output file which contains a record of the deployed workflow &#40;defaults to `WORKFLOW_NAME.yaml`&#41; where any spaces in the provided workflow name are replaced with `_`)

[//]: # ()
[//]: # (Note that the URLs must point to the raw files, not to the webpages for the Gists. Raw Gist URLs usually contain `/raw` or `/fileNameHere`. These URLs will be validated for correctness automatically before the workflow is deployed.)

[//]: # ()
[//]: # (### How to get a valid Github API token)

[//]: # ()
[//]: # (To generate, visit https://github.com/settings/tokens?type=beta and click "Generate new token". Name the token and enable read & write access for Gists from the "Account permissions" drop-down menu. Do not enable any additional permissions.)
