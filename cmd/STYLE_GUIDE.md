# CRE Style Guide

## Principles for CLI Design

### 1. **User-Friendly Onboarding**
- **Minimal Inputs**: Ask for the least amount of input possible. Provide sensible defaults where applicable to reduce the need for manual input.
- **Defaults & Overrides**: Use default values if an input is not specified. Allow users to override defaults via CLI or configuration files.
- **Bootstrapping process**: Help the user set up all necessary prerequisites before running any commands. Embed this process within the specialized initialize command.

### 2. **User Input Categories**
- **Sensitive Information**:
    - **Examples**: EOA private key, GitHub API key, ETH RPC URL, Secrets API key.
    - **Storage**: Store sensitive information securely, such as in 1Password.
- **Non-Sensitive Information**:
    - **Examples**: DON ID, Workflow registry address, Capabilities registry address, Workflow owner address, Log level, Seth config path.
    - **Storage**: Use a single YAML configuration file for non-sensitive data, and reference the secrets in 1Password within this configuration if needed.

### 3. **Configuration & Parameter Hierarchy**
- **Priority Order**:
    - CLI flags > configuration file > default values.
- **Handling Configuration**: Use [Viper](https://github.com/spf13/viper) to enforce this hierarchy and load settings effectively.

### 4. **Flag and Module Naming Conventions**
- **Kebab-Case**: Use kebab-case (e.g., `--binary-url`) for readability and consistency.
- **Short Form**: Provide a single lowercase letter for short-form flags where applicable (e.g., `-f`).
- **Module Naming**: Use kebab-case for module names as well (e.g., `compile-and-upload`).
- **Consistent Name**: Reuse flag names where possible, e.g. if you have `--binary-url` in one command, use the same flag for the second command.

### 5. **Flags vs. Positional Arguments**
- **Primary Argument**: If only one argument is mandatory, use it as positional argument (e.g., `cli workflow compile PATH_TO_FILE`).
- **Complex Commands**: If there are more than two required arguments, pick the most essential argument for positional argument. Others are flags (e.g., `cli workflow deploy WORKFLOW_NAME -binary-url=X`)..
- **Optional Fields**: Always represent optional fields as flags.

### 6. **Logging and Error Handling**
- **Verbosity Levels**: Default log level is INFO. Enable verbose logging (DEBUG/TRACE) with the `-v` flag.
- **Error Communication**: Catch errors and rewrite them in user-friendly terms, with guidance on next steps.
- **Progress Indicators**: For long-running operations, inform users with progress messages.

### 7. **Aborting and Exiting**
- **Graceful Exits**: Avoid fatal termination; print errors and exit gracefully.
- **Abort Signals**: Accept user signals (e.g., `Cmd+C`) to halt execution.

### 8. **Communication with the User**
- **Be Clear & Concise**: Avoid ambiguous messages and use simple and precise explanations. Don't overload the user with a ton of information.
- **Be Suggestive**: If an issue occurs, try to guide the user by suggesting how to fix it. If it's a success, inform the user about the next available steps (teach the user how to use the tool).
- **Accurate Help Docs**: The user must be able to easily find information on how to get help. CLI tool documentation must always reflect the current state of the tool.

### **Footnotes**
For additional guidance or future reference, please see the [CLI Guidelines](https://clig.dev/#guidelines) that inspired this documentation.
