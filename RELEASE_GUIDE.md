# Release Process

This project uses [Changesets](https://github.com/changesets/changesets) for versioning and release management.

## Adding a changeset

When your PR includes changes that warrant a version bump, run:

```bash
pnpm changeset
```

Select the bump type (major, minor, or patch) and provide a summary. This creates a `.changeset/*.md` file that should be committed with your PR.

## How releases happen

1. PRs with changeset files are merged to `main`.
2. The `release.yml` workflow detects pending changesets and opens (or updates) a **"Version Packages"** PR. This PR bumps `package.json`, updates `CHANGELOG.md`, and consumes the changeset files.
3. When the Version Packages PR is merged, the workflow creates a `v*` tag.
4. The tag triggers `build-and-release.yml`, which builds and signs binaries across all platforms and creates a draft GitHub Release.
5. Review the draft release and publish it.

## Hotfixing an existing release

1. Create a branch from the relevant release tag (e.g. `git checkout -b hotfix/v1.3.1 v1.3.0`).
2. Fix the bug and add a changeset (`pnpm changeset` -- typically a `patch`).
3. Follow the standard PR and merge process against `main`.
