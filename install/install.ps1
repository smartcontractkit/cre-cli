#
# PowerShell installer script for 'cre' on Windows.
# It detects the architecture, downloads the correct .exe,
# and adds it to the user's PATH.
#
# Usage: irm https://cre.chain.link/install.ps1 | iex

# --- Configuration ---
$ErrorActionPreference = "Stop" # Exit script on any error

$Repo = "smartcontractkit/cre-cli"
$CliName = "cre"
# Installation directory (user-specific, no admin rights needed)
$InstallDir = "$env:LOCALAPPDATA\Programs\$CliName"

# --- Main Installation Logic ---

try {
    # 1. Detect Architecture
    $Arch = $env:PROCESSOR_ARCHITECTURE
    switch ($Arch) {
        "AMD64" { $ArchName = "amd64" }
        "ARM64" { $ArchName = "arm64" }
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
    Write-Host "Latest version is $LatestTag."

    # 3. Construct Download URL and Destination Path
    $BinaryName = "$($CliName)-windows-$($ArchName).exe"
    $DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$BinaryName"
    $ExePath = Join-Path $InstallDir "$($CliName).exe"

    Write-Host "Downloading from $DownloadUrl..."

    # Create installation directory if it doesn't exist
    if (-not (Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # 4. Download the Binary
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath

    Write-Host "Successfully downloaded $CliName to $InstallDir."

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
    Write-Host "Run '$CliName --help' in a new terminal to get started."

} catch {
    Write-Host "Installation failed: $($_.Exception.Message)"
    exit 1
}