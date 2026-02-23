#!/usr/bin/env bash
set -euo pipefail

run_cmd() {
  local name="$1"
  shift
  if command -v "$1" >/dev/null 2>&1; then
    echo -n "${name}: "
    "$@" 2>/dev/null | head -n 1
  else
    echo "${name}: not-found"
  fi
}

echo "Date: $(date +%Y-%m-%d)"
echo "OS: $(uname -srm)"
echo "Terminal: ${TERM_PROGRAM:-unknown}"
run_cmd "Go" go version
run_cmd "Node" node --version
run_cmd "Bun" bun --version
run_cmd "Anvil" anvil --version

if [[ -x ./cre ]]; then
  echo -n "CRE: "
  ./cre version 2>/dev/null | head -n 1
else
  echo "CRE: ./cre binary not found"
fi
