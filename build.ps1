param(
    [switch]$SkipBuild,
    [switch]$Run,
    [switch]$Test,
    [switch]$FullOutput,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$WailsArgs
)

$ErrorActionPreference = "Stop"

# Force UTF-8 when reading output from external processes (Wails, go, node).
# Without this PowerShell uses the system ANSI code page (e.g. Windows-1252),
# which corrupts multi-byte characters like • and ♥ in Wails' output.
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

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
        (Join-Path $env:USERPROFILE "go\bin"),
        (Join-Path $env:SystemDrive "wails")
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
if ($SkipBuild -and $Test) {
    throw "Cannot use -SkipBuild and -Test together."
}
if ($Run -and $Test) {
    throw "Cannot use -Run and -Test together."
}

if ($SkipBuild) {
    Write-Host "Prerequisite check passed. Build skipped (-SkipBuild)." -ForegroundColor Green
    exit 0
}

Write-Host "Running: wails build" -ForegroundColor Cyan
if ($FullOutput) {
    & wails build @WailsArgs
} else {
    # Filter out Wails' INFO banners and sponsorship notice — they add noise
    # on every build but carry no actionable information.
    & wails build @WailsArgs 2>&1 | Where-Object { $_ -notmatch '\bINFO\b|sponsoring the project|leaanthony' }
}
if ($LASTEXITCODE -ne 0) {
    throw "wails build failed with exit code $LASTEXITCODE"
}

Write-Host "Build completed successfully." -ForegroundColor Green

if ($Test) {
    Write-Host "Running tests..." -ForegroundColor Cyan
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Tests failed."
    }
    Write-Host "All tests passed." -ForegroundColor Green
}

if ($Run) {
    $binaryPath = Join-Path $PSScriptRoot "build\bin\gitgo.exe"
    if (-not (Test-Path $binaryPath)) {
        throw "Build output not found at: $binaryPath"
    }
    Write-Host "Launching: $binaryPath" -ForegroundColor Cyan
    Start-Process -FilePath $binaryPath
}

Write-Host "Success." -ForegroundColor Green
