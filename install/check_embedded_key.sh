#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_sh="$repo_root/install/install.sh"
public_key="$repo_root/install/public_key.asc"
embedded_key="$(mktemp)"
trap 'rm -f "$embedded_key"' EXIT

sed -n '/^-----BEGIN PGP PUBLIC KEY BLOCK-----$/,/^-----END PGP PUBLIC KEY BLOCK-----$/p' "$install_sh" >"$embedded_key"

# Normalize trailing newlines so the check is stable across editors.
normalize_trailing_newline() {
  perl -0777 -ne 'print $_ =~ /\n\z/ ? $_ : "$_\n"' "$1"
}

if ! diff -u <(normalize_trailing_newline "$public_key") <(normalize_trailing_newline "$embedded_key"); then
  echo "install/install.sh embedded public key does not match install/public_key.asc" >&2
  exit 1
fi

echo "Embedded release public key matches install/public_key.asc"
