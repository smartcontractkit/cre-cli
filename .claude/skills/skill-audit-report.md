# Skill Audit Report

Date: 2026-02-23
Scope:
- `.claude/skills/using-cre-cli/SKILL.md`
- `.claude/skills/cre-add-template/SKILL.md`
- `.claude/skills/cre-qa-runner/SKILL.md`
- `.claude/skills/cre-cli-tui-testing/SKILL.md`

## Findings Summary

- `CRIT: 0`
- `WARN: 0`
- `INFO: 3`

## Notes

1. Invocation boundaries are explicit between command syntax (`using-cre-cli`) and PTY traversal (`cre-cli-tui-testing`).
2. `cre-add-template` and `cre-qa-runner` both mention dynamic template mode, but one is scoped to change workflow and the other to pre-release execution/reporting.
3. Dynamic template instructions are marked as branch-gated and do not override current embedded-template baseline behavior.

## Validation Method

- Trigger overlap check was completed via description scan (`rg -n "^description:" .claude/skills/*/SKILL.md`).
- Structural quality was reviewed directly in each `SKILL.md` and referenced files.
