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
        "ARM64" { $ArchName = "amd64" }
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
    $BinaryName = "$($CliName)_windows_$($ArchName).zip"
    $DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$BinaryName"

    # Use a temp directory for download and extraction
    $TempDir = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), "cre_install_" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    $ZipPath = Join-Path $TempDir "$($CliName).zip"

    $ProgressPreference = 'SilentlyContinue'
    Write-Host "Downloading from $DownloadUrl..."
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath

    Write-Host "Extracting $CliName.exe from zip..."
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($ZipPath, $TempDir)

    # Find the extracted exe (assume only one .exe in the zip)
    $ExtractedExe = Get-ChildItem -Path $TempDir -Filter "*.exe" | Select-Object -First 1
    if (-not $ExtractedExe) {
        throw "No .exe file found in the extracted zip archive."
    }

    # Create installation directory if it doesn't exist
    if (-not (Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # Copy the exe to the install directory and rename
    Copy-Item -Path $ExtractedExe.FullName -Destination (Join-Path $InstallDir "$($CliName).exe") -Force

    # Clean up temp directory
    Remove-Item -Path $TempDir -Recurse -Force

    Write-Host "Successfully extracted $CliName.exe to $InstallDir."

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