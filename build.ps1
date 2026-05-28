param(
    [switch]$SkipBuild,
    [switch]$Run,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$WailsArgs
)

$ErrorActionPreference = "Stop"

function Sync-PathFromRegistry {
    # Refresh the current session PATH so newly installed tools are visible
    # without requiring the user to open a new terminal.
    $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $combined = @($machinePath, $userPath) -join ";"

    if (-not [string]::IsNullOrWhiteSpace($combined)) {
        $env:Path = $combined
    }
}

function Add-CommonToolPaths {
    # Some installers (Node, Go) write to standard locations that may not yet
    # be reflected in the current terminal session PATH.
    $candidatePaths = @(
        "C:\Program Files\nodejs",
        "C:\Program Files\Go\bin",
        (Join-Path $env:USERPROFILE "go\bin")
    )

    foreach ($path in $candidatePaths) {
        if ((Test-Path $path) -and ($env:Path -notlike "*$path*")) {
            $env:Path = "$env:Path;$path"
        }
    }
}

function Require-Command {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    if (-not (Get-Command -Name $Name -ErrorAction SilentlyContinue)) {
        throw "Missing prerequisite: '$Name' is not installed or not on PATH."
    }
}

function Assert-GoVersion {
    $raw = (& go version)
    if ($raw -notmatch "go(?<major>\d+)\.(?<minor>\d+)") {
        throw "Could not parse Go version output: $raw"
    }

    $major = [int]$Matches.major
    $minor = [int]$Matches.minor

    if (($major -lt 1) -or ($major -eq 1 -and $minor -lt 25)) {
        throw "Go 1.25+ is required. Found: $raw"
    }
}

function Assert-NodeVersion {
    $raw = (& node --version).Trim()
    if ($raw -notmatch "^v(?<major>\d+)") {
        throw "Could not parse Node.js version output: $raw"
    }

    $major = [int]$Matches.major
    if ($major -lt 20) {
        throw "Node.js 20+ is required. Found: $raw"
    }
}

Sync-PathFromRegistry
Add-CommonToolPaths

Write-Host "Checking prerequisites..." -ForegroundColor Cyan
Require-Command -Name go
Require-Command -Name node
Require-Command -Name npm
Require-Command -Name wails

Assert-GoVersion
Assert-NodeVersion

$goVersion = (& go version)
$nodeVersion = (& node --version)
$npmVersion = (& npm --version)
$wailsVersionLines = (& wails version)
$wailsVersion = $wailsVersionLines | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -First 1

Write-Host "Go:     $goVersion"
Write-Host "Node:   $nodeVersion"
Write-Host "npm:    $npmVersion"
Write-Host "Wails:  $wailsVersion"

if ($SkipBuild -and $Run) {
    throw "Cannot use -SkipBuild and -Run together."
}

if ($SkipBuild) {
    Write-Host "Prerequisite check passed. Build skipped (-SkipBuild)." -ForegroundColor Green
    exit 0
}

Write-Host "Running: wails build" -ForegroundColor Cyan
& wails build @WailsArgs

Write-Host "Build completed successfully." -ForegroundColor Green

if ($Run) {
    $binaryPath = Join-Path $PSScriptRoot "build\bin\gitgo.exe"
    if (-not (Test-Path $binaryPath)) {
        throw "Build output not found at: $binaryPath"
    }
    Write-Host "Launching: $binaryPath" -ForegroundColor Cyan
    Start-Process -FilePath $binaryPath
}
