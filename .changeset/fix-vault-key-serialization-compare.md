---
"cre-cli": patch
---

Fix `cre secrets create` failing with "vault public key from gateway does not match CapabilitiesRegistry" when an Ethereum RPC is configured. The gateway and CapabilitiesRegistry serialize the same TDH2 vault public key differently (the gateway includes `HArray`), so the CLI now compares the decoded key components (Group, G_bar, H) instead of the raw hex strings.
