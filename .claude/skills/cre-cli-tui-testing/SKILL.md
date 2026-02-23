---
name: cre-cli-tui-testing
description: Runs repeatable CRE CLI interactive TUI traversal tests through PTY sessions, including wizard happy-path, cancel, validation, overwrite prompts, auth-gated interactive branches, and branch-gated dynamic-template browse/search failure scenarios. Use when the user asks to test Bubbletea wizard behavior, PTY/TTY input handling, or deterministic terminal traversal for CRE CLI interactive flows.
---

# CRE CLI TUI Testing

## Core Workflow

1. Confirm prerequisites and environment variables from `references/setup.md`.
2. Follow `references/test-flow.md` for the scenario sequence.
3. Use `tui_test/*.expect` for deterministic PTY tests.
4. Use `$playwright-cli` for browser-auth steps when requested.
5. For branch-gated dynamic template source paths, run browse/search and remote-error scenarios from `references/test-flow.md`.
6. Report exit status plus filesystem side effects for overwrite/cancel branches.

## Commands

```bash
# deterministic PTY happy-path traversal
expect ./.claude/skills/cre-cli-tui-testing/tui_test/pty-smoke.expect

# deterministic overwrite No/Yes branch checks
expect ./.claude/skills/cre-cli-tui-testing/tui_test/pty-overwrite.expect
```

## Notes

- Keep general command syntax questions in `$using-cre-cli`.
- This skill is specifically for interactive terminal behavior and traversal validation.
- Never print secret env values; check only whether required variables are set.
- Read `references/setup.md` before first run on a machine.
