#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_sh="$repo_root/install/install.sh"
failures=0

pass() { echo "PASS: $1"; }
fail_test() { echo "FAIL: $1"; failures=$((failures + 1)); }

# Source the tag-parsing helpers (extract_tag_name / is_valid_tag) from install.sh.
# shellcheck disable=SC1090
source <(sed -n '244,261p' "$install_sh")

assert_extract() {
  local name=$1
  local json=$2
  local want=$3
  local got

  got=$(extract_tag_name "$json")
  if [ "$got" = "$want" ]; then
    pass "$name"
  else
    fail_test "$name (got '$got', want '$want')"
  fi
}

assert_valid() {
  local name=$1
  local tag=$2

  if is_valid_tag "$tag"; then
    pass "$name"
  else
    fail_test "$name (expected valid, got rejected)"
  fi
}

assert_invalid() {
  local name=$1
  local tag=$2

  if is_valid_tag "$tag"; then
    fail_test "$name (expected rejected, got valid)"
  else
    pass "$name"
  fi
}

# --- extract_tag_name ---

assert_extract "pretty-printed JSON" \
  $'{\n  "tag_name": "v1.2.3",\n  "name": "Release v1.2.3"\n}' \
  "v1.2.3"

# Minified single-line JSON: the case the old grep|sed parser got wrong.
assert_extract "minified JSON" \
  '{"url":"https://example.com","tag_name":"v1.2.3","name":"other"}' \
  "v1.2.3"

assert_extract "tag_name not first field" \
  '{"id":42,"draft":false,"tag_name":"v4.5.6"}' \
  "v4.5.6"

# --- is_valid_tag ---

assert_valid "plain semver with v" "v1.2.3"
assert_valid "semver without v" "1.2.3"
assert_valid "pre-release suffix" "v1.2.3-rc.1"
assert_valid "build metadata suffix" "v1.2.3+build.7"

assert_invalid "empty string" ""
assert_invalid "the word latest" "latest"
assert_invalid "incomplete version" "v1"
assert_invalid "path traversal" "../../evil"
assert_invalid "command substitution" '$(rm -rf /)'
assert_invalid "url injection" "v1.2.3/../../etc"

if [ "$failures" -ne 0 ]; then
  echo "$failures tag parse test(s) failed" >&2
  exit 1
fi

echo "All tag parse tests passed."
