# Manual-Only Cases

These cases are not reliable to fully automate in a deterministic CLI-only run.

## Browser OAuth Flow

Cases:
- Initial browser login flow.
- Browser logout redirect confirmation.

Handling:
- If browser automation is not requested or not stable, mark `SKIP` with reason.
- If browser login is required for dependent steps and not available, mark dependent steps `BLOCKED`.
- Prefer API key auth for automated runs where acceptable.

## Visual Wizard Verification

Cases:
- Logo rendering quality.
- Color contrast and highlight visibility.
- Cross-terminal visual parity checks.

Handling:
- Mark as `SKIP` when running non-visual automation-only QA.
- Mark as `PASS`/`FAIL` only with explicit visual confirmation and terminal context.

## PTY-Specific Interactive Branches

Cases:
- Esc/Ctrl+C cancellation behavior.
- Overwrite prompt branch behavior.
- Auth-gated "Would you like to log in?" prompt interaction.

Handling:
- Route these checks through `$cre-cli-tui-testing` if deterministic PTY coverage is required.
