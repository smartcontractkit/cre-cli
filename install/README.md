# Installing the CRE CLI

This directory holds the bootstrap installers for the `cre` CLI:

- `install.sh` — Linux and macOS (bash)
- `install.ps1` — Windows (PowerShell)

## Quick install

**Linux / macOS**

```bash
curl -sSL https://app.chain.link/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://app.chain.link/install.ps1 | iex
```

This downloads the latest release, verifies the release **binary** signature
(GPG on Linux, `codesign` on macOS, Authenticode on Windows), and installs it.

## Verify the install script before running it

Piping a script straight into `bash`/`iex` runs it sight-unseen. If you would
rather verify the bootstrap script itself first, every GitHub release publishes
`install.sh`, `install.ps1`, and a `checksums.txt` containing their SHA-256
digests. Download, verify, inspect, then run the local copy.

**Linux / macOS**

```bash
tag=$(curl -fsSL https://api.github.com/repos/smartcontractkit/cre-cli/releases/latest \
  | grep -Eo '"tag_name":\s*"[^"]+"' | head -n1 | grep -Eo 'v[0-9][^"]*')
base="https://github.com/smartcontractkit/cre-cli/releases/download/$tag"

curl -fsSL "$base/install.sh"    -o install.sh
curl -fsSL "$base/checksums.txt" -o checksums.txt

# Compare the published digest against the file you downloaded.
expected=$(grep '^install.sh:' checksums.txt | awk '{print $2}')
actual=$(shasum -a 256 install.sh | awk '{print $1}')
[ "$expected" = "$actual" ] && echo "OK: checksum matches" || { echo "MISMATCH"; exit 1; }

# Inspect, then run the verified local copy.
less install.sh
bash install.sh
```

**Windows (PowerShell)**

```powershell
$tag  = (Invoke-RestMethod https://api.github.com/repos/smartcontractkit/cre-cli/releases/latest).tag_name
$base = "https://github.com/smartcontractkit/cre-cli/releases/download/$tag"

Invoke-WebRequest "$base/install.ps1"    -OutFile install.ps1
Invoke-WebRequest "$base/checksums.txt"  -OutFile checksums.txt

$expected = ((Select-String '^install.ps1:' checksums.txt).Line -split '\s+')[1]
$actual   = (Get-FileHash -Algorithm SHA256 install.ps1).Hash.ToLower()
if ($expected -eq $actual) { "OK: checksum matches" } else { throw "MISMATCH" }

# Inspect, then run the verified local copy.
Get-Content install.ps1
.\install.ps1
```

> **Note:** `checksums.txt` uses a `filename: sha256` format (not the
> `sha256  filename` layout that `sha256sum -c` expects), so verify by comparing
> the digest as shown above rather than piping into `sha256sum -c`.

## What is verified, and what is not

- **Release binaries** are cryptographically verified by the install script:
  GPG signature (Linux, against the key embedded in `install.sh` /
  `install/public_key.asc`), Apple `codesign` (macOS), and Authenticode
  (Windows).
- **The bootstrap scripts** are served from `app.chain.link`. Verifying them
  against the release-published `checksums.txt` (above) confirms they match the
  reviewed source for a given release tag.
