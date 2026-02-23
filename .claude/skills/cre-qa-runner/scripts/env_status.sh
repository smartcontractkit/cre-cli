#!/usr/bin/env bash
set -euo pipefail

vars=(CRE_API_KEY ETH_PRIVATE_KEY CRE_ETH_PRIVATE_KEY CRE_CLI_ENV)

for v in "${vars[@]}"; do
  if [[ -n "${!v-}" ]]; then
    echo "${v}=set"
  else
    echo "${v}=unset"
  fi
done
