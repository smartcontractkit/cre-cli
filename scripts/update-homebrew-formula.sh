#!/usr/bin/env bash
#
# Update Formula/cre.rb with version and checksums from a published GitHub release.
#
# Usage:
#   ./scripts/update-homebrew-formula.sh v1.24.0
#   ./scripts/update-homebrew-formula.sh   # uses latest published release

set -euo pipefail

REPO="smartcontractkit/cre-cli"
FORMULA_FILE="Formula/cre.rb"
GITHUB_API="https://api.github.com/repos/${REPO}"

fail() {
  echo "Error: $1" >&2
  exit 1
}

TAG="${1:-}"
if [[ -z "${TAG}" ]]; then
  TAG="$(curl -fsSL "${GITHUB_API}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
fi

[[ "${TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Invalid tag format: ${TAG} (expected vX.Y.Z)"

VERSION="${TAG#v}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
CHECKSUMS="$(curl -fsSL "${CHECKSUMS_URL}")"

lookup_checksum() {
  local versioned_name="$1"
  local checksum
  checksum="$(printf '%s\n' "${CHECKSUMS}" | grep "^${versioned_name}:" | head -1 | awk '{print $2}')"
  [[ -n "${checksum}" ]] || fail "Checksum not found for ${versioned_name}"
  printf '%s' "${checksum}"
}

DARWIN_ARM64_SHA="$(lookup_checksum "cre_v${VERSION}_darwin_arm64.zip")"
DARWIN_AMD64_SHA="$(lookup_checksum "cre_v${VERSION}_darwin_amd64.zip")"
LINUX_ARM64_SHA="$(lookup_checksum "cre_v${VERSION}_linux_arm64.tar.gz")"
LINUX_AMD64_SHA="$(lookup_checksum "cre_v${VERSION}_linux_amd64.tar.gz")"

cat > "${FORMULA_FILE}" <<EOF
class Cre < Formula
  desc "Chainlink Runtime Environment CLI"
  homepage "https://chain.link/chainlink-runtime-environment"
  version "${VERSION}"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/${TAG}/cre_darwin_arm64.zip"
      sha256 "${DARWIN_ARM64_SHA}"
    end
    on_intel do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/${TAG}/cre_darwin_amd64.zip"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/${TAG}/cre_linux_arm64.tar.gz"
      sha256 "${LINUX_ARM64_SHA}"
    end
    on_intel do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/${TAG}/cre_linux_amd64.tar.gz"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    arch = Hardware::CPU.arm? ? "arm64" : "amd64"
    platform = OS.mac? ? "darwin" : "linux"
    bin.install Dir["cre_v*_#{platform}_#{arch}"].first => "cre"
  end

  def caveats
    <<~EOS
      Go 1.25.3 or later and Bun 1.0.0 or later are recommended for developing
      and running TypeScript CRE workflows.
    EOS
  end

  test do
    assert_match "CRE CLI version v#{version}", shell_output("#{bin}/cre version")
  end
end
EOF

echo "Updated ${FORMULA_FILE} for ${TAG}"
