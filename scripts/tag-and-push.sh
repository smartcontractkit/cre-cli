#!/bin/bash
set -euo pipefail

version=$(node -p "require('./package.json').version")
tag="v${version}"

if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "Tag $tag already exists. Skipping..."
  exit 0
fi

sha=$(git rev-parse HEAD)
repo="${GITHUB_REPOSITORY}"

echo "Creating signed tag: $tag (pointing to $sha)"

# Create an annotated tag object via the GitHub API.
# Tags created server-side are signed by GitHub and display as "Verified".
tag_sha=$(gh api "repos/${repo}/git/tags" \
  --method POST \
  -f tag="$tag" \
  -f message="CRE CLI $tag" \
  -f object="$sha" \
  -f type="commit" \
  --jq '.sha')

# Create the ref pointing to the tag object
gh api "repos/${repo}/git/refs" \
  --method POST \
  -f ref="refs/tags/$tag" \
  -f sha="$tag_sha"

echo "Tag $tag created and verified."
