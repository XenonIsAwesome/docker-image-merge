# Cross-platform installer for docker-image-merge (Windows).
#
# Downloads the latest GitHub release binary and installs it as a Docker CLI plugin.
#
# Usage (PowerShell):
#   iwr -useb https://raw.githubusercontent.com/XenonIsAwesome/docker-image-merge/main/scripts/install.ps1 | iex
#   .\install.ps1                    # install to $env:USERPROFILE\.docker\cli-plugins
#   .\install.ps1 -System           # install to $env:ProgramData\Docker\cli-plugins

param(
    [switch]$System,
    [string]$Dir,
    [switch]$Help
)

$ErrorActionPreference = "Stop"
$Repo = "XenonIsAwesome/docker-image-merge"
$Binary = "docker-imagemerge"
$CliPluginsUser = Join-Path $env:USERPROFILE ".docker\cli-plugins"
$CliPluginsSystem = Join-Path $env:ProgramData "Docker\cli-plugins"

if ($Help) {
    Write-Host "Usage: .\install.ps1 [--system] [--dir <path>]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -System    Install system-wide to $CliPluginsSystem"
    Write-Host "  -Dir PATH  Install to a custom directory"
    Write-Host "  -Help      Show this help"
    exit 0
}

# Determine install directory.
$InstallDir = if ($System) { $CliPluginsSystem } elseif ($Dir) { $Dir } else { $CliPluginsUser }

# Detect architecture.
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Host "Error: 32-bit systems are not supported"
    exit 1
}

$OS = "windows"
Write-Host "Detected: $OS/$Arch"

# Resolve latest release.
Write-Host "Fetching latest release..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $Latest = $release.tag_name
} catch {
    Write-Host "Error: could not determine latest release"
    exit 1
}

$Version = if ($env:VERSION) { $env:VERSION } else { $Latest }
Write-Host "Installing $Version for $OS/$Arch..."

# Build download URL.
$ArchiveName = "docker-imagemerge-$OS-$Arch.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$ArchiveName"

# Download to temp directory.
$Staging = Join-Path ([System.IO.Path]::GetTempPath()) "docker-imagemerge-install"
if (Test-Path $Staging) { Remove-Item -Recurse -Force $Staging }
New-Item -ItemType Directory -Path $Staging | Out-Null

$ArchivePath = Join-Path $Staging "archive.zip"
Write-Host "Downloading $DownloadUrl ..."
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing
} catch {
    Write-Host "Error: download failed"
    Write-Host "URL: $DownloadUrl"
    exit 1
}

# Extract.
Write-Host "Extracting..."
Expand-Archive -Path $ArchivePath -DestinationPath $Staging -Force

# Find the binary.
$BinaryPath = Get-ChildItem -Path $Staging -Recurse -Filter "$Binary*" | Select-Object -First 1
if (-not $BinaryPath) {
    Write-Host "Error: binary $Binary not found in archive"
    Get-ChildItem -Path $Staging -Recurse
    exit 1
}

# Install.
Write-Host "Installing to $InstallDir\$Binary ..."
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path $BinaryPath.FullName -Destination (Join-Path $InstallDir "$Binary.exe") -Force

# Verify.
$InstalledBinary = Join-Path $InstallDir "$Binary.exe"
try {
    & $InstalledBinary --help | Out-Null
    Write-Host ""
    Write-Host "Installed successfully!"
    Write-Host "  $InstalledBinary"
    Write-Host ""
    Write-Host "Usage: docker imagemerge <image-a> <image-b> <output-image>"
} catch {
    Write-Host ""
    Write-Host "Installed: $InstalledBinary"
    Write-Host "(could not verify - Docker may not be running)"
}

# Cleanup.
Remove-Item -Recurse -Force $Staging
