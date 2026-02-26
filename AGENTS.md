# AGENTS.md

## Repository Purpose

CRE CLI source repository for command implementation, docs, and test flows across project init, auth, workflow lifecycle, and secrets management.

## Key Paths

- CLI docs: `docs/*.md`
- Testing framework docs: `testing-framework/*.md`
- CLI commands: `cmd/`
- Core internals: `internal/`
- E2E/integration tests: `test/`
- Local skills: `.claude/skills/`
- External template clone config: `submodules.yaml`
- External template setup script: `scripts/setup-submodules.sh`

## `cre-templates` Relationship

- `cre-templates` is configured in `submodules.yaml` under `submodules.cre-templates` with upstream `https://github.com/smartcontractkit/cre-templates.git` and branch `main`.
- This repo does **not** use Git submodules for `cre-templates` (`scripts/setup-submodules.sh` explicitly treats these as regular clones into gitignored directories).
- `make setup-submodules`, `make update-submodules`, and `make clean-submodules` call `scripts/setup-submodules.sh` to clone/update/remove the local `cre-templates/` checkout.
- The clone target is auto-added to `.gitignore` by the setup script (managed section).
- Runtime scaffolding for `cre init` uses embedded templates in this repo (`cmd/creinit/template/workflow/**/*` via `go:embed`), so `cre-templates` is an external reference/workspace dependency, not the direct runtime source for CLI template generation.

## Template Source Modes

- Current baseline (active): embedded templates from `cmd/creinit/template/workflow/**/*` are compiled into the CLI.
- Upcoming mode (branch-gated): dynamic template pull from the external template repository is planned but not baseline behavior yet.
- Until dynamic mode lands, treat dynamic-template guidance as preparation-only documentation and skill logic.

## Dynamic-Mode Workflow (When Branch Is Active)

1. Record which source mode was used for every init/simulate validation (embedded vs dynamic).
2. Capture template provenance for dynamic mode (repo, branch/ref, commit SHA if available).
3. Validate CLI-template compatibility across Linux, macOS, and Windows for the selected template source.
4. Re-run `skill-auditor` on touched skills before merge to keep invocation boundaries clear.

## Repository Component Map

```
                               USER / AGENT INPUT
                                        |
                                        v
                            +----------------------+
                            |   CLI Entrypoint     |
                            |      main.go         |
                            +----------+-----------+
                                       |
                                       v
                           +------------------------+
                           | Cobra Commands         |
                           | cmd/*                  |
                           | (init, workflow, etc.) |
                           +-----------+------------+
                                       |
                  +--------------------+--------------------+
                  |                                         |
                  v                                         v
       +--------------------------+              +--------------------------+
       | Internal Runtime/Logic   |              | User-Facing Docs         |
       | internal/*               |              | docs/cre_*.md            |
       | auth, clients, settings, |              | command flags/examples   |
       | validation, UI/TUI       |              +--------------------------+
       +------------+-------------+
                    |
                    v
       +--------------------------+
       | External Surfaces        |
       | GraphQL/Auth0/Chain RPC, |
       | storage, Vault DON       |
       +------------+-------------+
                    |
                    v
       +--------------------------+
       | Test Layers              |
       | test/*                   |
       | unit + e2e + PTY/TUI     |
       +------------+-------------+
                    |
                    v
       +--------------------------+
       | Skill Layer              |
       | .claude/skills/*         |
       | usage/testing/auditing   |
       +--------------------------+
```

## Component Interaction Flow

```
docs/*.md -> command intent -> cmd/* execution -> internal/* behavior
                                           |
                                           +-> interactive prompts (Bubbletea/TUI)
                                           +-> API/auth/network integrations

test/* validates cmd/* + internal/* behavior
.claude/skills/* guides agents on docs navigation, PTY/TUI traversal, browser steps, and skill quality checks
```

## Skill Map

- `using-cre-cli`
  - Use for command syntax, flags, and command-to-doc navigation.
- `cre-cli-tui-testing`
  - Use for PTY/TUI traversal validation, deterministic interactive flows, and auth-gated prompt checks.
- `playwright-cli`
  - Use for browser automation tasks, including CRE login page traversal when browser steps are required.
- `skill-auditor`
  - Use to audit skill quality, invocation accuracy, and structure after skill creation/updates.
- `cre-qa-runner`
  - Use for pre-release or release-candidate QA execution across the full runbook, with structured report generation.
- `cre-add-template`
  - Use when adding or modifying CRE init templates to enforce registry, test, and documentation checklist coverage.

## CLI Navigation Workflow

1. Identify the command area (`init`, `workflow`, `secrets`, `account`, `auth`).
2. Read the corresponding `docs/cre_*.md` file.
3. Use `using-cre-cli` for exact command/flag guidance.
4. For interactive wizard/auth prompt behavior, use `cre-cli-tui-testing`.
5. For browser-only steps (OAuth pages), use `playwright-cli`.

## TTY and PTY Notes

- Coding agents in this environment are already TTY-capable.
- No extra headless-terminal tooling is required for baseline interactive CLI traversal.
- Deterministic PTY flows are in `.claude/skills/cre-cli-tui-testing/tui_test/`.
- `expect` is optional but recommended for deterministic local replay.

## Prerequisites

For TUI + auth automation workflows, see:
- `.claude/skills/cre-cli-tui-testing/references/setup.md`

Do not print raw secret values. Report only set/unset status for env vars.

## Maintenance

When command behavior, prompts, or docs change:
1. Update affected `docs/cre_*.md` files if needed.
2. Update `using-cre-cli`, `cre-cli-tui-testing`, `cre-qa-runner`, and/or `cre-add-template` skill references.
3. Re-run `skill-auditor` on modified skills.
