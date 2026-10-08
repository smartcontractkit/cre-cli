---
"cre-cli": patch
---

Fix telemetry for commands that don't require login (`cre generate-bindings`, `cre workflow build`, `cre workflow hash`, etc.). Credentials are now attached silently when they exist on disk, so usage events from logged-in users are sent instead of being dropped. Commands remain fully usable while logged out.
