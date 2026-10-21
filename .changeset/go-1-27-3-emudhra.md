---
"cre-cli": patch
---

Build the CLI with Go 1.27.3, which distrusts the eight compromised eMudhra (emSign) intermediate CAs (CVE-2026-111943). Earlier builds accept TLS certificates issued from those keys, so an attacker on the network path could impersonate the CRE API, login, Vault gateway or RPC endpoints the CLI connects to. Release builds now take the Go version from `go.mod`.
