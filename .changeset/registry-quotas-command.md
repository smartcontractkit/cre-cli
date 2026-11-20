---
"cre-cli": minor
---

Add `cre registry quotas [registry-id]`, which shows how many workflows are active against each DON family's limit in every registry available to your organization, and how many owner addresses are linked or pending against the linked-owner limit. Supports `--json`. If part of the quotas cannot be read, the rest is still shown with a warning. `cre registry list` now points to the new command.
