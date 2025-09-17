# How to create a new release
Let's assume we want to create a release for version `v0.1.0`.

Steps:
1. Create a new branch from `main` with the name `releases/v0.1.x`
2. Create a tag on the release branch `git tag -a v0.1.0 -m "Release v0.1.0" -s`
3. Push the tag to the remote `git push origin v0.1.0`
4. Wait for `build-and-release` pipeline to run.
    - Once pipeline is successful, the release will be created as a Draft
    - Verify all is good and publish release as needed (set as latest if that is the case)

# How to fix a bug in an existing release
Let's assume we want to fix a bug in the release `v0.1.0`.

Steps:
1. Create a new branch from `releases/v0.1.x`.
2. Fix the bug. 
3. Create a PR against the branch `releases/v0.1.x`.
4. Go through the review process and merge the PR.
5. Create a release with a tag `v0.1.1` from the branch `releases/v0.1.x`.
