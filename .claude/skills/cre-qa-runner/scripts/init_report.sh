#!/usr/bin/env bash
set -euo pipefail

template=".qa-test-report-template.md"
report_date="${1:-$(date +%Y-%m-%d)}"
out=".qa-test-report-${report_date}.md"

if [[ ! -f "${template}" ]]; then
  echo "ERROR: Missing ${template}" >&2
  exit 1
fi

cp "${template}" "${out}"

required_headers=("## Run Metadata" "## 2. Build & Smoke Test" "## Summary")
for h in "${required_headers[@]}"; do
  if ! rg -q "^${h}$" "${out}"; then
    echo "ERROR: Report is missing required heading: ${h}" >&2
    exit 1
  fi
done

echo "Created report: ${out}"
