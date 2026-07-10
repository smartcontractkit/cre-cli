# Homebrew Core Prerequisites

This document tracks blockers for submitting `cre` to [homebrew-core](https://github.com/Homebrew/homebrew-core) so users can run `brew install cre` without a tap.

The in-repo formula at [Formula/cre.rb](../Formula/cre.rb) is the supported distribution path until these prerequisites are met.

## Current blockers

| Blocker | Status | Notes |
|---|---|---|
| Notability | Blocked | Homebrew requires ~225 GitHub stars for self-submitted software. cre-cli currently has low adoption relative to that threshold. |
| Source build | Blocked | Production builds use `CGO_ENABLED=1` and a GitHub token for `smartcontractkit/*` modules. Homebrew CI cannot use private credentials. |
| Go version alignment | Blocked | `go.mod` specifies Go 1.26.4 while CI/release workflows use Go 1.25.x. Versions should be aligned before submission. |
| Stable test output | Ready | `cre version` prints `CRE CLI version vX.Y.Z`, suitable for a `test do` block. |

## Required before opening a homebrew-core PR

1. Grow project notability (stars, forks, documented adoption).
2. Ensure `go build` succeeds on a clean machine without `GITHUB_TOKEN` (all modules publicly resolvable).
3. Document and satisfy any CGO/system library requirements for source builds.
4. Align Go versions across `go.mod`, CI, and release workflows.
5. Fork `homebrew/homebrew-core`, add `Formula/c/cre.rb` as a **source-build** formula, and pass:
   - `brew audit --new-formula --strict cre`
   - `brew install --build-from-source cre`
   - `brew test cre`

## Source-build formula sketch

When the blockers above are resolved, use a source-build formula similar to:

```ruby
class Cre < Formula
  desc "Chainlink Runtime Environment CLI"
  homepage "https://chain.link/chainlink-runtime-environment"
  url "https://github.com/smartcontractkit/cre-cli/archive/refs/tags/vX.Y.Z.tar.gz"
  sha256 "..."
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/smartcontractkit/cre-cli/cmd/version.Version=version v#{version}"
    system "go", "build", *std_go_args(ldflags:), "."
  end

  test do
    assert_match "CRE CLI version v#{version}", shell_output("#{bin}/cre version")
  end
end
```

## Submission checklist

1. Confirm all blockers above are resolved.
2. Open a PR titled `cre X.Y.Z (new formula)` against `homebrew/homebrew-core`.
3. Respond to maintainer review feedback until CI passes.
