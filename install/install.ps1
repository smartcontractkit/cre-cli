#
# PowerShell installer script for 'cre' on Windows.
# It detects the architecture, downloads the correct .exe,
# and adds it to the user's PATH.
#
# Usage: irm https://app.chain.link/install.ps1 | iex

# --- Configuration ---
$ErrorActionPreference = "Stop" # Exit script on any error

$Repo    = "smartcontractkit/cre-cli"
$CliName = "cre"
# Installation directory (user-specific, no admin rights needed)
$InstallDir = "$env:LOCALAPPDATA\Programs\$CliName"

# === Version Requirements for Workflow Dependencies ===
# These do NOT block CLI installation; they are used to print helpful warnings.
$RequiredGoVersion   = "1.25.3"
$RequiredGoMajor     = 1
$RequiredGoMinor     = 25

# Choose a conservative Bun floor for TS workflows.
$RequiredBunVersion  = "1.0.0"
$RequiredBunMajor    = 1
$RequiredBunMinor    = 0

# --- Helper Functions ---

function Fail {
    param(
        [string]$Message
    )
    Write-Host "Error: $Message" -ForegroundColor Red
    exit 1
}

function Test-GoDependency {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Warning "'go' is not installed."
        Write-Host "         Go $RequiredGoVersion or later is recommended to build CRE Go workflows."
        return
    }

    # Example: "go version go1.25.3 windows/amd64"
    $output = go version 2>$null
    if (-not $output) {
        Write-Warning "Could not determine Go version. Go $RequiredGoVersion or later is recommended for CRE Go workflows."
        return
    }

    $tokens = $output -split ' '
    if ($tokens.Length -lt 3) {
        Write-Warning "Unexpected 'go version' output: '$output'. Go $RequiredGoVersion or later is recommended."
        return
    }

    $ver = $tokens[2] -replace '^go', ''  # remove leading 'go'
    if (-not $ver) {
        Write-Warning "Could not parse Go version from '$output'. Go $RequiredGoVersion or later is recommended."
        return
    }

    $parts = $ver.Split('.')
    if ($parts.Count -lt 2) {
        Write-Warning "Could not parse Go version '$ver'. Go $RequiredGoVersion or later is recommended."
        return
    }

    [int]$goMajor = $parts[0]
    [int]$goMinor = $parts[1]

    if (($goMajor -lt $RequiredGoMajor) -or
       (($goMajor -eq $RequiredGoMajor) -and ($goMinor -lt $RequiredGoMinor))) {
        Write-Warning "Detected Go $ver."
        Write-Host  "         Go $RequiredGoVersion or later is recommended to build CRE Go workflows."
    }
}

function Test-BunDependency {
    if (-not (Get-Command bun -ErrorAction SilentlyContinue)) {
        Write-Warning "'bun' is not installed."
        Write-Host "         Bun $RequiredBunVersion or later is recommended to run TypeScript CRE workflows (e.g. 'postinstall: bun x cre-setup')."
        return
    }

    # Bun version examples:
    #  - "1.2.1"
    #  - "bun 1.2.1"
    $output = bun -v 2>$null | Select-Object -First 1
    if (-not $output) {
        Write-Warning "Could not determine Bun version. Bun $RequiredBunVersion or later is recommended for TypeScript workflows."
        return
    }

    $ver = $output.Trim() -replace '^bun\s+', ''
    if (-not $ver) {
        Write-Warning "Could not parse Bun version from '$output'. Bun $RequiredBunVersion or later is recommended."
        return
    }

    $parts = $ver.Split('.')
    if ($parts.Count -lt 2) {
        Write-Warning "Could not parse Bun version '$ver'. Bun $RequiredBunVersion or later is recommended."
        return
    }

    [int]$bunMajor = $parts[0]
    [int]$bunMinor = $parts[1]

    if (($bunMajor -lt $RequiredBunMajor) -or
       (($bunMajor -eq $RequiredBunMajor) -and ($bunMinor -lt $RequiredBunMinor))) {
        Write-Warning "Detected Bun $ver."
        Write-Host  "         Bun $RequiredBunVersion or later is recommended to run TypeScript CRE workflows."
    }
}

function Test-ReleaseAuthenticode {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath
    )

    $signature = Get-AuthenticodeSignature -FilePath $FilePath
    if ($signature.Status -ne 'Valid') {
        Fail "authenticode status: $($signature.Status)"
    }
    if (-not $signature.SignerCertificate) {
        Fail "missing signer certificate"
    }
    if ($signature.SignerCertificate.Subject -notlike '*SmartContract*') {
        Fail "unexpected signer: $($signature.SignerCertificate.Subject)"
    }
}

function Test-ValidTag {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Tag
    )

    # Fail closed on anything that is not a plausible release tag
    # (vMAJOR.MINOR.PATCH with optional pre-release/build suffix).
    if ($Tag -notmatch '^v?\d+\.\d+\.\d+([-.+][0-9A-Za-z.-]+)*$') {
        Fail "Refusing to install: invalid release tag '$Tag'."
    }
}

function Test-UnsafeZipEntry {
    param(
        [Parameter(Mandatory = $true)]
        [string]$EntryName
    )

    if ($EntryName -match '\.\.') {
        return $true
    }
    if ($EntryName -match '^[/\\]') {
        return $true
    }
    return $false
}

function Extract-ExpectedZipEntry {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ZipPath,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedEntryName,
        [Parameter(Mandatory = $true)]
        [string]$DestPath
    )

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $matches = @()
        foreach ($entry in $zip.Entries) {
            if (Test-UnsafeZipEntry -EntryName $entry.FullName) {
                throw "Unsafe zip entry: $($entry.FullName)"
            }
            if ($entry.Name -eq $ExpectedEntryName) {
                $matches += $entry
            }
        }

        if ($matches.Count -ne 1) {
            throw "Expected exactly one zip entry named $ExpectedEntryName, found $($matches.Count)."
        }

        $entry = $matches[0]
        $parent = Split-Path -Parent $DestPath
        if (-not (Test-Path -Path $parent)) {
            New-Item -ItemType Directory -Path $parent | Out-Null
        }

        $entryStream = $entry.Open()
        try {
            $destStream = [System.IO.File]::Create($DestPath)
            try {
                $entryStream.CopyTo($destStream)
            } finally {
                $destStream.Dispose()
            }
        } finally {
            $entryStream.Dispose()
        }
    } finally {
        $zip.Dispose()
    }
}

try {
    # 1. Detect Architecture
    $Arch = $env:PROCESSOR_ARCHITECTURE
    switch ($Arch) {
        "AMD64" { $ArchName = "amd64" }
        "ARM64" { $ArchName = "amd64" } # currently use amd64 build for ARM64 Windows
        default { throw "Unsupported architecture: $Arch" }
    }
    Write-Host "Detected Windows on $ArchName architecture."

    # 2. Get Latest Release Version from GitHub
    Write-Host "Fetching the latest version of $CliName..."
    $ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
    $LatestRelease = Invoke-RestMethod -Uri $ApiUrl
    $LatestTag = $LatestRelease.tag_name
    if (-not $LatestTag) {
        throw "Could not determine the latest release tag from GitHub."
    }
    Test-ValidTag $LatestTag
    Write-Host "Latest version is $LatestTag."

    # 3. Construct Download URL and Destination Path
    $BinaryName = "$($CliName)_windows_$($ArchName).zip"
    $DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$BinaryName"

    # Use a temp directory for download and extraction
    $TempDir = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), "cre_install_" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    $ZipPath = Join-Path $TempDir "$($CliName).zip"

    $ProgressPreference = 'SilentlyContinue'
    Write-Host "Downloading from $DownloadUrl..."
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath

    $ExpectedExeName = "$($CliName)_$($LatestTag)_windows_$($ArchName).exe"
    $ExtractedExePath = Join-Path $TempDir $ExpectedExeName

    Write-Host "Extracting $ExpectedExeName from zip..."
    Extract-ExpectedZipEntry -ZipPath $ZipPath -ExpectedEntryName $ExpectedExeName -DestPath $ExtractedExePath

    Write-Host "Verifying release signature..."
    Test-ReleaseAuthenticode -FilePath $ExtractedExePath

    # Create installation directory if it doesn't exist
    if (-not (Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # Copy the exe to the install directory and rename
    $ExePath = Join-Path $InstallDir "$($CliName).exe"
    Copy-Item -Path $ExtractedExePath -Destination $ExePath -Force

    # Clean up temp directory
    Remove-Item -Path $TempDir -Recurse -Force

    Write-Host "Successfully extracted $CliName.exe to $InstallDir."

    # 4. Verify the binary runs
    try {
        & $ExePath version | Out-Null
    } catch {
        throw "$CliName installation failed when running '$CliName version'."
    }

    # 5. Add to User's PATH
    Write-Host "Adding '$InstallDir' to your PATH."

    # Get the current user's PATH
    $UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")

    # Add the install directory to the PATH if it's not already there
    if (-not ($UserPath -split ';' -contains $InstallDir)) {
        $NewPath = "$InstallDir;$UserPath"
        [System.Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "'$InstallDir' has been added to your PATH."
        Write-Host "Please restart your terminal or open a new one for the changes to take effect."
    } else {
        Write-Host "'$InstallDir' is already in your PATH."
    }

    Write-Host ""
    Write-Host "$CliName was installed successfully! 🎉"
    Write-Host ""

    # 6. Post-install dependency checks (Go & Bun)
    Write-Host "Performing environment checks for CRE workflows..."
    Test-GoDependency
    Test-BunDependency
    Write-Host ""
    Write-Host "If you plan to build Go workflows, ensure Go >= $RequiredGoVersion."
    Write-Host "If you plan to build TypeScript workflows, ensure Bun >= $RequiredBunVersion."
    Write-Host ""
    Write-Host "Run '$CliName --help' in a new terminal to get started."

} catch {
    Write-Host "Installation failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
