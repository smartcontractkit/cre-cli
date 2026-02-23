#!/usr/bin/env bash
set -euo pipefail

changed="$(git status --porcelain | awk '{print $2}')"

require_match() {
  local pattern="$1"
  local label="$2"
  if echo "${changed}" | rg -q "${pattern}"; then
    echo "OK: ${label}"
  else
    echo "MISSING: ${label}" >&2
    return 1
  fi
}

status=0

require_match '^cmd/creinit/template/workflow/' 'template files under cmd/creinit/template/workflow/' || status=1
require_match '^cmd/creinit/creinit.go$' 'template registry update in cmd/creinit/creinit.go' || status=1
require_match '^test/template_compatibility_test.go$' 'compatibility test update in test/template_compatibility_test.go' || status=1

if echo "${changed}" | rg -q '^docs/'; then
  echo 'OK: docs updates detected'
else
  echo 'MISSING: docs updates under docs/' >&2
  status=1
fi

if [[ "${status}" -ne 0 ]]; then
  echo 'Template gap check failed.' >&2
  exit 1
fi

echo 'Template gap check passed.'
