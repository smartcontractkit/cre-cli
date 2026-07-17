package install

import _ "embed"

// ReleasePublicKey is the Linux GPG public key used to verify cre release binaries.
//
//go:embed public_key.asc
var ReleasePublicKey []byte
