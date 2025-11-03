<div style="text-align:center" align="center">
    <a href="https://chain.link" target="_blank">
        <img src="https://raw.githubusercontent.com/smartcontractkit/chainlink/develop/docs/logo-chainlink-blue.svg" width="225" alt="Chainlink logo">
    </a>

[![License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/smartcontractkit/cre-cli/blob/main/README.md)
[![CRE Documentation](https://img.shields.io/static/v1?label=CRE&message=Home&color=blue)](https://chain.link/chainlink-runtime-environment)

</div>

# Chainlink Runtime Environment (CRE) - CLI Tool

> If you want to **write workflows**, please use the public documentation: https://docs.chain.link/cre  
> This README is intended for **CRE CLI developers** (maintainers/contributors), not CRE end users.

A Go/Cobra-based command-line tool for building, testing, and managing Chainlink Runtime Environment (CRE) workflows. This repository contains the CLI source code and developer tooling.

- [Installation](#installation)
- [Developer Commands](#developer-commands)
- [CRE Commands](#commands)
- [Legal Notice](#legal-notice)

## Installation

1. Clone the repository:

    ```bash
    git clone https://github.com/smartcontractkit/cre-cli.git
    cd cre-cli
    ````

2. Make sure you have Go installed. You can check this with:

   ```bash
   go version
   ```

## Developer Commands

Developer commands are available via the Makefile:

* **Install dependencies/tools**

  ```bash
  make install-tools
  ```

* **Build the binary (for local testing)**

  ```bash
  make build
  ```

* **Run linters**

  ```bash
  make lint
  ```

* **Regenerate CLI docs (when commands/flags change)**

  ```bash
  make gendoc
  ```

## Commands

For a list of all commands and their descriptions, please refer to the [docs](docs) folder.

## Legal Notice

By using the CRE CLI tool, you agree to the Terms of Service ([https://chain.link/terms](https://chain.link/terms)) and Privacy Policy ([https://chain.link/privacy-policy](https://chain.link/privacy-policy)).
