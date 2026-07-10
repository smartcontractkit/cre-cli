#!/usr/bin/env bash
#
# This is a universal installer script for 'cre'.
# It detects the OS and architecture, then downloads the correct binary.
#
# Usage: curl -sSL https://app.chain.link/install.sh | bash

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

# CRE_RELEASE_PUBLIC_KEY - must stay in sync with install/public_key.asc (prodsec-owned).
write_release_public_key() {
  local key_file=$1
  cat >"$key_file" <<'CRE_RELEASE_PUBLIC_KEY'
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGjJhYEDEACcUl1IfB6dKn5VhvbP2LOctUwq/qr81RZmpdADixM9183GJ1JR
2mO0abYAuEdqsA8px7vhkgcL/yigMgGKrWbKzV+Zvcrb9+fLQoLxvG5KVFil7zJn
+9qwmxsvjlUB8VmikxyXBO7E7uxVj2iJWZX22R/k9d4t1lmHG9ewBf+5A7oNrU1P
j8nN0N0UwFgcJp70A5y6j20AJ97SFhUtVGtcaClObtCiPryVUgttdqOniIDBHJQH
OAhQdZmkYYCjnMT/sJyuY7IZzPoJhRioU60pBD/mJPgdwdl0rxCby/lAPffQIh2+
yakKnHerb6Y1T8m1Xjj+iawRcr6Y9Cr4yhoRawqX1F3DziK2/35RNIY24JyXP8/y
hADOk/gHkigepl17pbZmGIDTUoZEUVb1iRo1/3x9PVWOP1FCbRvJ9B1PrcbwCcZ3
5bTReX77ZyIrirapAZ2cCqFiTxxaAgoYAZrcOBr7dlJlPJYue84nzLSFnA1llBHc
/KN8IYf5Qud6HEMs9hO4ESCT+CKy2Ng9H4WpiA8ArL0txOIkJ5mx26CavFbyJRnk
IUrQ4HxXgj12yQetBNbI7ysJMuaLneomva3HuQ7zb9/chC05qiaQWXDhQpvKN+0i
AvXiSPU5B29LNzFQO6Srxh0an1Rl+kakNlZMbN4pXvMiVoxR/ecBkPowNwARAQAB
tDNDUkUgPGNyZUBzbWFydGNvbnRyYWN0LmNvbT4gKExpbnV4IEdQRyBTaWduaW5n
IEtleSmJAnMEEAMJAF0FAmjJhYE0HENSRSA8Y3JlQHNtYXJ0Y29udHJhY3QuY29t
PiAoTGludXggR1BHIFNpZ25pbmcgS2V5KRYhBIJEkQLLuJ7ZvuEaSEZFvBQEwvPd
Ah4DAhsDBBUICQoACgkQRkW8FATC890a8Q/+LJXi64znLkZei+VK7wQzlRW6Mo13
U2BzNd5ZWMqWrU5LomSU4uDHQkGBEx9CCCfRcIv6bdbM9Iga6yzFSIv8HZxgb6tL
bP2/Ly9+cgqUGTQ5ChBrs68DiTxAS4skSSP7ap5pL/TLp+Qc8AUN2XhlRJz0HLTO
bZoz5fBTKBOBAKNz2zu8uWYdijx9cX1YPr2HsuT/HF9dcmSDRXY0nSkvebWcSN8s
Tu/g22eBrQkiNRjqsRuxdxG1SQHL+Qq5DK6xRc7KUVaZCjBTnGLXaMPhFxwZkvW7
PTa21XRW/f2bTDR/vxpjwN9n5yFOxnm4pWiJEW1jodXzIhMNDqsJTGsk+N+4kT8k
0rAoHd0D1pmo8jQXLG2FldP359JDfZMR10S1Lv7uBhPsgj9vUA4uWsy7Prf5H9zo
JTQ3B/xVi0LYYKveu/Nm3VvXJY53vfAWmIn6s0iLrTrlSrZuglfK70HnMJv5a6jc
BcyE563wmJKVjLK8ZqggPYdOaeVZfy0k4wmyupVjNU0O5GxMUg7dANmtu8bDHUBU
MBo06MuHpthkdM1DHxnpBLw0YsWzumpEpVZatASWfZ5o1pxm/PB4KR6rsY4bdnoD
wUlRxURvO/I2jQJPacYrw24pb7ufRs9MXqQEUEbSpXRBs5CbBADw2qRcr7vrZnze
a8cULyg4Y65LBeU=
=cKUx
-----END PGP PUBLIC KEY BLOCK-----
CRE_RELEASE_PUBLIC_KEY
}

is_unsafe_archive_entry() {
  local entry=$1
  case "$entry" in
    ../* | */../* | */.. | .. | /* | \\*)
      return 0
      ;;
  esac
  return 1
}

safe_extract_archive() {
  local archive_path=$1
  local dest_dir=$2
  local expected_member=$3

  if echo "$archive_path" | grep -qE '\.tar\.gz$'; then
    check_command "tar"
    local members
    members=$(tar -tzf "$archive_path") || fail "Failed to list tar archive members."

    local member_count=0
    local matched_member=""
    while IFS= read -r entry; do
      [ -n "$entry" ] || continue
      if is_unsafe_archive_entry "$entry"; then
        fail "Unsafe tar entry: $entry"
      fi
      member_count=$((member_count + 1))
      if [ "$entry" = "$expected_member" ]; then
        matched_member=$entry
      elif [ "${entry%/}" = "$expected_member" ]; then
        matched_member="${entry%/}"
      fi
    done <<<"$members"

    if [ "$member_count" -ne 1 ]; then
      fail "Expected exactly one archive member, found $member_count."
    fi
    if [ -z "$matched_member" ]; then
      fail "Expected archive member $expected_member not found."
    fi

    tar -xzf "$archive_path" -C "$dest_dir" "$matched_member" ||
      fail "Failed to extract $matched_member from archive."
    return
  fi

  if echo "$archive_path" | grep -qE '\.zip$'; then
    check_command "unzip"
    local members
    members=$(unzip -Z1 "$archive_path") || fail "Failed to list zip archive members."

    local member_count=0
    local matched_member=""
    while IFS= read -r entry; do
      [ -n "$entry" ] || continue
      if is_unsafe_archive_entry "$entry"; then
        fail "Unsafe zip entry: $entry"
      fi
      member_count=$((member_count + 1))
      if [ "$entry" = "$expected_member" ]; then
        matched_member=$entry
      elif [ "${entry%/}" = "$expected_member" ]; then
        matched_member="${entry%/}"
      fi
    done <<<"$members"

    if [ "$member_count" -ne 1 ]; then
      fail "Expected exactly one archive member, found $member_count."
    fi
    if [ -z "$matched_member" ]; then
      fail "Expected archive member $expected_member not found."
    fi

    unzip -oq "$archive_path" -d "$dest_dir" "$matched_member" ||
      fail "Failed to extract $matched_member from archive."
    return
  fi

  fail "Unknown archive format: $archive_path"
}

verify_linux_gpg_signature() {
  local bin_path=$1
  local sig_path=$2
  local key_file=$3

  check_command "gpg"

  gpg --batch --import "$key_file" >/dev/null 2>&1 ||
    fail "Failed to import release public key."

  local gpg_out
  gpg_out=$(gpg --batch --status-fd=1 --verify "$sig_path" "$bin_path" 2>&1) ||
    fail "GPG signature verification failed."

  echo "$gpg_out" | grep -q '\[GNUPG:\] VALIDSIG' ||
    fail "GPG signature verification failed: no valid signature."
  echo "$gpg_out" | grep -q 'cre@smartcontract.com' ||
    fail "GPG signature verification failed: unexpected signer."
}

verify_darwin_codesign() {
  local bin_path=$1

  codesign --verify --strict --identifier com.smartcontract.cre.cli "$bin_path" ||
    fail "codesign verification failed."
}

verify_release_binary() {
  local bin_path=$1
  local sig_path=$2
  local key_file=$3

  case "$PLATFORM" in
    linux)
      verify_linux_gpg_signature "$bin_path" "$sig_path" "$key_file"
      ;;
    darwin)
      verify_darwin_codesign "$bin_path"
      ;;
    *)
      fail "Unsupported platform for release verification: $PLATFORM"
      ;;
  esac
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

if command -v brew >/dev/null 2>&1; then
  echo "Homebrew detected. You can install cre with:"
  echo "  brew tap smartcontractkit/cre-cli https://github.com/smartcontractkit/cre-cli"
  echo "  brew install cre"
  echo
fi

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
trap 'rm -rf "$TMP_DIR"' EXIT
ARCHIVE_PATH="$TMP_DIR/$ASSET"

curl --fail --location --progress-bar "$DOWNLOAD_URL" --output "$ARCHIVE_PATH" || fail "Failed to download asset from $DOWNLOAD_URL"

EXPECTED_BIN_NAME="${cli_name}_${LATEST_TAG}_${PLATFORM}_${ARCH_NAME}"
TMP_CRE_BIN="$TMP_DIR/$EXPECTED_BIN_NAME"
PUBLIC_KEY_FILE="$TMP_DIR/public_key.asc"
SIG_PATH=""

if [ "$PLATFORM" = "linux" ]; then
  check_command "gpg"
  SIG_ASSET="${cli_name}_${PLATFORM}_${ARCH_NAME}.sig"
  SIG_URL="https://github.com/$github_repo/releases/download/$LATEST_TAG/$SIG_ASSET"
  SIG_PATH="$TMP_DIR/$SIG_ASSET"
  curl --fail --location --progress-bar "$SIG_URL" --output "$SIG_PATH" ||
    fail "Failed to download signature from $SIG_URL"
  write_release_public_key "$PUBLIC_KEY_FILE"
fi

# 5. Extract archive and verify release authenticity before install
safe_extract_archive "$ARCHIVE_PATH" "$TMP_DIR" "$EXPECTED_BIN_NAME"

[ -f "$TMP_CRE_BIN" ] || fail "Binary $TMP_CRE_BIN not found after extraction."

verify_release_binary "$TMP_CRE_BIN" "$SIG_PATH" "$PUBLIC_KEY_FILE"

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
