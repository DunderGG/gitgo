<#
.SYNOPSIS
    Creates a test git repository under testdata/ for development and manual testing.

.DESCRIPTION
    Sets up two directories:
        testdata/test-remote/  — a bare repository simulating a remote server
        testdata/test-repo/    — a working repository with:
                                   3 pushed commits  (already on the fake remote)
                                   3 unpushed commits (local only, editable in GitGo)

    Re-running this script resets both directories to a clean state.
#>

$ErrorActionPreference = "Stop"

$testDataDir   = Join-Path $PSScriptRoot "testdata"
$testRepoDir   = Join-Path $testDataDir  "test-repo"
$testRemoteDir = Join-Path $testDataDir  "test-remote"

# ── Clean up any previous run ──────────────────────────────────────────────────
if (Test-Path $testRepoDir)   { Remove-Item $testRepoDir   -Recurse -Force }
if (Test-Path $testRemoteDir) { Remove-Item $testRemoteDir -Recurse -Force }

# ── Create bare remote ─────────────────────────────────────────────────────────
Write-Host "Creating bare remote  : $testRemoteDir" -ForegroundColor Cyan
git init --bare $testRemoteDir | Out-Null

# ── Create working repository ──────────────────────────────────────────────────
Write-Host "Creating working repo : $testRepoDir" -ForegroundColor Cyan
git init $testRepoDir -b main | Out-Null

Push-Location $testRepoDir

# Use local config so the test repo does not depend on the global git config.
git config user.name        "Test User"
git config user.email       "testuser@example.com"
git config core.autocrlf    false

git remote add origin $testRemoteDir

# Helper function — write a file, stage it, and commit with a fixed date.
function Add-Commit {
    param(
        [string]$CommitMessage,
        [string]$FileName,
        [string]$FileContent,
        [string]$CommitDate
    )

    Set-Content -Path $FileName -Value $FileContent -Encoding UTF8 -NoNewline
    git add $FileName | Out-Null
    $env:GIT_AUTHOR_DATE    = $CommitDate
    $env:GIT_COMMITTER_DATE = $CommitDate
    git commit -m $CommitMessage --quiet
}

# ── Pushed commits (will be on the remote after the push below) ────────────────

Add-Commit `
    -CommitMessage "Initial commit" `
    -FileName      "README.md" `
    -FileContent   "# Test Project`n`nA sample repository for GitGo testing." `
    -CommitDate    "2026-01-10T09:00:00"

Add-Commit `
    -CommitMessage "Add main source file" `
    -FileName      "main.go" `
    -FileContent   "package main`n`nfunc main() {}`n" `
    -CommitDate    "2026-01-15T14:30:00"

Add-Commit `
    -CommitMessage "Expand README with usage section" `
    -FileName      "README.md" `
    -FileContent   "# Test Project`n`nA sample repository for GitGo testing.`n`n## Usage`n`nClone and run.`n" `
    -CommitDate    "2026-02-03T11:00:00"

# Push so these three commits are considered upstream (pushed).
git push --set-upstream origin main --quiet
Write-Host "Pushed 3 commits to remote." -ForegroundColor DarkGray

# ── Unpushed commits (local only — these are the ones GitGo can edit) ──────────

git config user.name  "Alice Dev"
git config user.email "alice@example.com"

Add-Commit `
    -CommitMessage "Update README: add contributing guide" `
    -FileName      "README.md" `
    -FileContent   "# Test Project`n`nA sample repository for GitGo testing.`n`n## Usage`n`nClone and run.`n`n## Contributing`n`nPull requests welcome.`n" `
    -CommitDate    "2026-05-20T10:00:00"

Add-Commit `
    -CommitMessage "Refactor: rename entry point function" `
    -FileName      "main.go" `
    -FileContent   "package main`n`nfunc main() {`n`trun()`n}`n`nfunc run() {}`n" `
    -CommitDate    "2026-05-25T16:45:00"

Add-Commit `
    -CommitMessage "Add configuration file" `
    -FileName      "config.json" `
    -FileContent   "{`n  `"version`": `"0.1.0`",`n  `"debug`": false`n}`n" `
    -CommitDate    "2026-05-27T09:30:00"

# Clear the date overrides.
$env:GIT_AUTHOR_DATE    = $null
$env:GIT_COMMITTER_DATE = $null

Pop-Location

Write-Host ""
Write-Host "Test repository ready." -ForegroundColor Green
Write-Host "  Working repo : $testRepoDir"
Write-Host "  Bare remote  : $testRemoteDir"
Write-Host ""
Write-Host "Commit log:" -ForegroundColor Cyan
git -C $testRepoDir log --oneline
