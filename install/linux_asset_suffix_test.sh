#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_sh="$repo_root/install/install.sh"
failures=0

pass() { echo "PASS: $1"; }
fail_test() { echo "FAIL: $1"; failures=$((failures + 1)); }

# shellcheck disable=SC1090
source <(sed -n '198,242p' "$install_sh")

assert_parse_version() {
  local name=$1
  local output=$2
  local want=$3

  if got=$(parse_glibc_version_from_ldd_output "$output"); then
    if [ "$got" = "$want" ]; then
      pass "$name"
    else
      fail_test "$name (got $got, want $want)"
    fi
  else
    fail_test "$name (parse failed)"
  fi
}

assert_suffix() {
  local name=$1
  local output=$2
  local want=$3
  local version suffix

  PLATFORM=linux
  if ! version=$(parse_glibc_version_from_ldd_output "$output"); then
    suffix=""
  elif version_lt "$version" "$LINUX_GLIBC_THRESHOLD"; then
    suffix="$LINUX_LDD235_SUFFIX"
  else
    suffix=""
  fi

  if [ "$suffix" = "$want" ]; then
    pass "$name"
  else
    fail_test "$name (got '$suffix', want '$want')"
  fi
}

assert_parse_version "ubuntu 22.04" \
  $'ldd (Ubuntu GLIBC 2.35-0ubuntu3.8) 2.35\nCopyright (C) 2022 Free Software Foundation, Inc.\n' \
  "2.35"

assert_parse_version "ubuntu 24.04" \
  $'ldd (Ubuntu GLIBC 2.39-0ubuntu8.4) 2.39\n' \
  "2.39"

assert_parse_version "rhel style" \
  $'ldd (GNU libc) 2.34\n' \
  "2.34"

if parse_glibc_version_from_ldd_output "" >/dev/null 2>&1; then
  fail_test "empty output rejects parse"
else
  pass "empty output rejects parse"
fi

assert_suffix "ubuntu 22.04 uses ldd2-35" \
  $'ldd (Ubuntu GLIBC 2.35-0ubuntu3.8) 2.35\n' \
  "_ldd2-35"

assert_suffix "ubuntu 24.04 uses default asset" \
  $'ldd (Ubuntu GLIBC 2.39-0ubuntu8.4) 2.39\n' \
  ""

assert_suffix "rhel 2.34 uses ldd2-35" \
  $'ldd (GNU libc) 2.34\n' \
  "_ldd2-35"

assert_suffix "empty output fails open" "" ""

if [ "$failures" -ne 0 ]; then
  echo "$failures linux asset suffix test(s) failed" >&2
  exit 1
fi

echo "All linux asset suffix tests passed."
