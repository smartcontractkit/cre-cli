#!/bin/bash
set -euo pipefail

version=$(node -p "require('./package.json').version")
tag="v${version}"

if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "Tag $tag already exists. Skipping..."
  exit 0
fi

echo "Creating and pushing tag: $tag"
git tag -a "$tag" -m "CRE CLI $tag"
git push origin "$tag"
