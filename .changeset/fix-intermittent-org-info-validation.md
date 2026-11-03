---
"cre-cli": patch
---

Fix intermittent "unable to retrieve organization info" errors during credential validation. Non-2xx responses from the CRE API are now reported with their HTTP status instead of being decoded as empty organization data, transient API failures (429, 5xx) are retried with backoff, and the error now states which organization fields were missing.
