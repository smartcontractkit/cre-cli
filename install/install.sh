#!/usr/bin/env bash
#
# This is a universal installer script for 'cre'.
# It detects the OS and architecture, then downloads the correct binary.
#
# Usage: curl -sSL https://cre.chain.link/install.sh | bash

set -e # Exit immediately if a command exits with a non-zero status.

# === Version Requirements for Workflow Dependencies ===
# These do NOT block CLI installation; they are used to print helpful warnings.
REQUIRED_GO_VERSION="1.25.3"
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=25

# Choose a conservative Bun floor for TS workflows.
REQUIRED_BUN_VERSION="1.0.0"
REQUIRED_BUN_MAJOR=1
REQUIRED_BUN_MINOR=0

# --- Helper Functions ---

# Function to print error messages and exit.
fail() {
  echo "Error: $1" >&2
  exit 1
}

# Function to check for required commands.
check_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command '$1' is not installed."
}

tildify() {
    if [[ $1 = $HOME/* ]]; then
        local replacement=\~/

        echo "${1/$HOME\//$replacement}"
    else
        echo "$1"
    fi
}

# Check Go dependency and version (for Go-based workflows).
check_go_dependency() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Warning: 'go' is not installed."
    echo "         Go $REQUIRED_GO_VERSION or later is recommended to build CRE Go workflows."
    return
  fi

  # Example output: 'go version go1.25.3 darwin/arm64'
  go_version_str=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
  if [ -z "$go_version_str" ]; then
    echo "Warning: Could not determine Go version. Go $REQUIRED_GO_VERSION or later is recommended for CRE Go workflows."
    return
  fi

  go_major=${go_version_str%%.*}
  go_minor_patch=${go_version_str#*.}
  go_minor=${go_minor_patch%%.*}

  if [ "$go_major" -lt "$REQUIRED_GO_MAJOR" ] || \
     { [ "$go_major" -eq "$REQUIRED_GO_MAJOR" ] && [ "$go_minor" -lt "$REQUIRED_GO_MINOR" ]; }; then
    echo "Warning: Detected Go $go_version_str."
    echo "         Go $REQUIRED_GO_VERSION or later is recommended to build CRE Go workflows."
  fi
}

# Check Bun dependency and version (for TypeScript workflows using 'bun x cre-setup').
check_bun_dependency() {
  if ! command -v bun >/dev/null 2>&1; then
    echo "Warning: 'bun' is not installed."
    echo "         Bun $REQUIRED_BUN_VERSION or later is recommended to run TypeScript CRE workflows (e.g. 'postinstall: bun x cre-setup')."
    return
  fi

  # Bun version examples:
  #  - '1.2.1'
  #  - 'bun 1.2.1'
  bun_version_str=$(bun -v 2>/dev/null | head -n1)
  bun_version_str=${bun_version_str#bun }

  if [ -z "$bun_version_str" ]; then
    echo "Warning: Could not determine Bun version. Bun $REQUIRED_BUN_VERSION or later is recommended for TypeScript workflows."
    return
  fi

  bun_major=${bun_version_str%%.*}
  bun_minor_patch=${bun_version_str#*.}
  bun_minor=${bun_minor_patch%%.*}

  if [ "$bun_major" -lt "$REQUIRED_BUN_MAJOR" ] || \
     { [ "$bun_major" -eq "$REQUIRED_BUN_MAJOR" ] && [ "$bun_minor" -lt "$REQUIRED_BUN_MINOR" ]; }; then
    echo "Warning: Detected Bun $bun_version_str."
    echo "         Bun $REQUIRED_BUN_VERSION or later is recommended to run TypeScript CRE workflows."
  fi
}

# --- Main Installation Logic ---

# 1. Define Variables
github_repo="smartcontractkit/cre-cli"
cli_name="cre"

install_env=CRE_INSTALL
bin_env=\$$install_env/bin

install_dir=${!install_env:-$HOME/.cre}
bin_dir=$install_dir/bin
cre_bin=$bin_dir/$cli_name

# 2. Detect OS and Architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)
    PLATFORM="linux"
    ;;
  Darwin) # macOS
    PLATFORM="darwin"
    ;;
  *)
    fail "Unsupported operating system: $OS. For Windows, please use the PowerShell script."
    ;;
esac

case "$ARCH" in
  x86_64 | amd64)
    ARCH_NAME="amd64"
    ;;
  arm64 | aarch64)
    ARCH_NAME="arm64"
    ;;
  *)
    fail "Unsupported architecture: $ARCH"
    ;;
esac

if [[ ! -d $bin_dir ]]; then
    mkdir -p "$bin_dir" ||
        fail "Failed to create install directory \"$bin_dir\""
fi

# 3. Determine the Latest Version from GitHub Releases
check_command "curl"
LATEST_TAG=$(curl -s "https://api.github.com/repos/$github_repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
  fail "Could not fetch the latest release version from GitHub."
fi

if [[ $# = 0 ]]; then
  echo "Installing $cli_name version $LATEST_TAG for $PLATFORM/$ARCH_NAME..."
else
  LATEST_TAG=$1
fi

# 4. Construct Download URL and Download asset
ASSET="${cli_name}_${PLATFORM}_${ARCH_NAME}"
# Determine the file extension based on OS
if [ "$PLATFORM" = "linux" ]; then
  ASSET="${ASSET}.tar.gz"
elif [ "$PLATFORM" = "darwin" ]; then
  ASSET="${ASSET}.zip"
fi
DOWNLOAD_URL="https://github.com/$github_repo/releases/download/$LATEST_TAG/$ASSET"

TMP_DIR=$(mktemp -d)
ARCHIVE_PATH="$TMP_DIR/$ASSET"

curl --fail --location --progress-bar "$DOWNLOAD_URL" --output "$ARCHIVE_PATH" || fail "Failed to download asset from $DOWNLOAD_URL"

# 5. Extract archive and locate the binary
if echo "$ASSET" | grep -qE '\.tar\.gz$'; then
  check_command "tar"
  tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"
elif echo "$ASSET" | grep -qE '\.zip$'; then
  check_command "unzip"
  unzip -oq "$ARCHIVE_PATH" -d "$TMP_DIR"
else
  fail "Unknown archive format: $ASSET"
fi

TMP_CRE_BIN="$TMP_DIR/${cli_name}_${LATEST_TAG}_${PLATFORM}_${ARCH_NAME}"

[ -f "$TMP_CRE_BIN" ] || fail "Binary $TMP_CRE_BIN not found after extraction."
chmod +x "$TMP_CRE_BIN"

# 6. Install the Binary (moving into place)
if [ -w "$install_dir" ]; then
  mv "$TMP_CRE_BIN" "$cre_bin"
else
  echo "Write permission to $install_dir denied. Attempting with sudo..."
  check_command "sudo"
  sudo mv "$TMP_CRE_BIN" "$cre_bin"
fi

# 7. Check that the binary runs
"$cre_bin" version || fail "$cli_name installation failed."

# Cleanup
rm -rf "$TMP_DIR"

# 8. Post-install dependency checks (Go & Bun)
echo
echo "Performing environment checks for CRE workflows..."
check_go_dependency
check_bun_dependency
echo

refresh_command=''

tilde_bin_dir=$(tildify "$bin_dir")
quoted_install_dir=\"${install_dir//\"/\\\"}\"

if [[ $quoted_install_dir = \"$HOME/* ]]; then
    quoted_install_dir=${quoted_install_dir/$HOME\//\$HOME/}
fi

case $(basename "$SHELL") in
fish)
    commands=(
        "set --export $install_env $quoted_install_dir"
        "set --export PATH $bin_env \$PATH"
    )

    fish_config=$HOME/.config/fish/config.fish
    tilde_fish_config=$(tildify "$fish_config")

    if [[ -w $fish_config ]]; then
        if ! grep -q "# cre" "$fish_config"; then
            {
                echo -e '\n# cre'
                for command in "${commands[@]}"; do
                    echo "$command"
                done
            } >>"$fish_config"
        fi

        echo "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_fish_config\""

        refresh_command="source $tilde_fish_config"
    else
        echo "Manually add the directory to $tilde_fish_config (or similar):"

        for command in "${commands[@]}"; do
            echo "  $command"
        done
    fi
    ;;
zsh)
    commands=(
        "export $install_env=$quoted_install_dir"
        "export PATH=\"$bin_env:\$PATH\""
    )

    zsh_config=$HOME/.zshrc
    tilde_zsh_config=$(tildify "$zsh_config")

    if [[ -w $zsh_config ]]; then
      if ! grep -q "# cre" "$zsh_config"; then
        {
            echo -e '\n# cre'

            for command in "${commands[@]}"; do
                echo "$command"
            done
        } >>"$zsh_config"
      fi

      echo "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_zsh_config\""

      refresh_command="exec $SHELL"
    else
        echo "Manually add the directory to $tilde_zsh_config (or similar):"

        for command in "${commands[@]}"; do
            echo "  $command"
        done
    fi
    ;;
bash)
    commands=(
        "export $install_env=$quoted_install_dir"
        "export PATH=\"$bin_env:\$PATH\""
    )

    bash_configs=(
        "$HOME/.bash_profile"
        "$HOME/.bashrc"
    )

    if [[ ${XDG_CONFIG_HOME:-} ]]; then
        bash_configs+=(
            "$XDG_CONFIG_HOME/.bash_profile"
            "$XDG_CONFIG_HOME/.bashrc"
            "$XDG_CONFIG_HOME/bash_profile"
            "$XDG_CONFIG_HOME/bashrc"
        )
    fi

    set_manually=true
    for bash_config in "${bash_configs[@]}"; do
        tilde_bash_config=$(tildify "$bash_config")

        if [[ -w $bash_config ]]; then
          if ! grep -q "# cre" "$bash_config"; then
            {
                echo -e '\n# cre'

                for command in "${commands[@]}"; do
                    echo "$command"
                done
            } >>"$bash_config"
          fi

          echo "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_bash_config\""

          refresh_command="source $bash_config"
          set_manually=false
          break
        fi
    done

    if [[ $set_manually = true ]]; then
        echo "Manually add the directory to $tilde_bash_config (or similar):"

        for command in "${commands[@]}"; do
            echo "  $command"
        done
    fi
    ;;
*)
    echo 'Manually add the directory to ~/.bashrc (or similar):'
    echo "  export $install_env=$quoted_install_dir"
    echo "  export PATH=\"$bin_env:\$PATH\""
    ;;
esac

echo
echo "$cli_name was installed successfully to $install_dir/$cli_name"
echo
echo "To get started, run:"
echo

if [[ $refresh_command ]]; then
    echo "  $refresh_command"
fi

echo "  $cli_name --help"
echo
echo "If you plan to build Go workflows, ensure Go >= $REQUIRED_GO_VERSION."
echo "If you plan to build TypeScript workflows, ensure Bun >= $REQUIRED_BUN_VERSION."
