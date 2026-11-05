---
"cre-cli": patch
---

Fix intermittent "unable to retrieve organization info" errors during credential validation. Non-2xx responses from the CRE API are now reported with their HTTP status instead of being decoded as empty organization data, transient failures (429, 5xx, connection resets, timeouts) are retried with jittered backoff, and the error now states which organization fields were missing.
