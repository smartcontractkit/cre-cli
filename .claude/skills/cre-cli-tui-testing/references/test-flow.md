# Test Flow

## Scenario order

1. Happy path wizard traversal
2. Cancel path (`Esc`)
3. Invalid input validation
4. Existing-directory overwrite prompt (`No` then `Yes`)
5. Optional auth-prompt branch (`y`/`n`)
6. Optional browser login completion via `$playwright-cli`
7. Branch-gated dynamic-template browse/search success path (when dynamic source flags exist)
8. Branch-gated dynamic-template remote failure path (network/auth/ref mismatch), with expected error classification

## Deterministic scripts

```bash
expect ./.claude/skills/cre-cli-tui-testing/tui_test/pty-smoke.expect
expect ./.claude/skills/cre-cli-tui-testing/tui_test/pty-overwrite.expect
```

## Manual PTY fallback

```bash
script -q /dev/null ./cre init
```

## Browser auth note

- Use `cre login` to emit a fresh authorize URL.
- Drive the browser flow with `$playwright-cli` only when browser automation is explicitly requested.
- Verify completion with `cre whoami`.

## Dynamic template note

- Run scenarios 7-8 only when dynamic template source behavior is available in the active branch.
- Record source mode and any remote ref details in test notes.
