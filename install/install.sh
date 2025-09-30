#!/bin/sh
#
# This is a universal installer script for 'cre'.
# It detects the OS and architecture, then downloads the correct binary.
#
# Usage: curl -sSL https://cre.chain.link/install.sh | sh

set -e # Exit immediately if a command exits with a non-zero status.

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

# --- Main Installation Logic ---

# 1. Define Variables
REPO="smartcontractkit/cre-cli" # Your GitHub repository
CLI_NAME="cre"
INSTALL_DIR="/usr/local/bin"

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

# 3. Determine the Latest Version from GitHub Releases
check_command "curl"
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
  fail "Could not fetch the latest release version from GitHub."
fi

# 4. Construct Download URL and Download asset
ASSET="${CLI_NAME}_${PLATFORM}_${ARCH_NAME}"
# Determine the file extension based on OS
if [ "$PLATFORM" = "linux" ]; then
  ASSET="${ASSET}.tar.gz"
elif [ "$PLATFORM" = "darwin" ]; then
  ASSET="${ASSET}.zip"
fi
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$ASSET"

echo "Downloading $CLI_NAME ($LATEST_TAG) for $PLATFORM/$ARCH_NAME from $DOWNLOAD_URL"

# Use curl to download the asset to a temporary file
TMP_DIR=$(mktemp -d)
curl -fSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ASSET" || fail "Failed to download asset from $DOWNLOAD_URL"

# Extract if it's a tar.gz
if echo "$ASSET" | grep -qE '\.tar\.gz$'; then
  tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
  TMP_FILE="$TMP_DIR/$ASSET"
  echo "Extracted to $TMP_FILE"
fi

# Extract if it's a zip
if echo "$ASSET" | grep -qE '\.zip$'; then
  check_command "unzip"
  unzip -o "$TMP_DIR/$ASSET" -d "$TMP_DIR"
  TMP_FILE="$TMP_DIR/$ASSET"
fi

BINARY_FILE="$TMP_DIR/${CLI_NAME}_${LATEST_TAG}_${PLATFORM}_${ARCH_NAME}"
# 5. Install the Binary
echo "Installing $CLI_NAME to $INSTALL_DIR"
[ -f "$TMP_FILE" ] || fail "Temporary file $TMP_FILE does not exist."
chmod +x "$TMP_FILE"

# Check for write permissions and use sudo if necessary
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY_FILE" "$INSTALL_DIR/$CLI_NAME"
else
  echo "Write permission to $INSTALL_DIR denied. Attempting with sudo..."
  check_command "sudo"
  sudo mv "$BINARY_FILE" "$INSTALL_DIR/$CLI_NAME"
fi

# check if the binary is installed correctly
$CLI_NAME version || fail "$CLI_NAME installation failed."

#cleanup
rm -rf "$TMP_DIR"

echo "$CLI_NAME installed successfully! Run '$CLI_NAME --help' to get started."