#!/usr/bin/env bash
# Creates a test git repository under testdata/ for development and manual testing.
#
# Sets up two directories:
#   testdata/test-remote/  — a bare repository simulating a remote server
#   testdata/test-repo/    — a working repository with:
#                              3 pushed commits  (already on the fake remote)
#                              3 unpushed commits (local only, editable in GitGo)
#
# Re-running this script resets both directories to a clean state.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/testdata"
TEST_REPO_DIR="$TESTDATA_DIR/test-repo"
TEST_REMOTE_DIR="$TESTDATA_DIR/test-remote"

# ── Clean up any previous run ──────────────────────────────────────────────────
rm -rf "$TEST_REPO_DIR" "$TEST_REMOTE_DIR"

# ── Create bare remote ─────────────────────────────────────────────────────────
echo "Creating bare remote  : $TEST_REMOTE_DIR"
git init --bare "$TEST_REMOTE_DIR" --quiet

# ── Create working repository ──────────────────────────────────────────────────
echo "Creating working repo : $TEST_REPO_DIR"
git init "$TEST_REPO_DIR" --quiet --initial-branch=main

cd "$TEST_REPO_DIR"

# Use local config so the test repo does not depend on the global git config.
git config user.name  "Test User"
git config user.email "testuser@example.com"

git remote add origin "$TEST_REMOTE_DIR"

# Helper: write a file, stage it, and commit with a fixed date.
add_commit() {
    local commit_message="$1"
    local file_name="$2"
    local file_content="$3"
    local commit_date="$4"

    printf '%s' "$file_content" > "$file_name"
    git add "$file_name"
    GIT_AUTHOR_DATE="$commit_date" GIT_COMMITTER_DATE="$commit_date" \
        git commit -m "$commit_message" --quiet
}

# ── Pushed commits (will be on the remote after the push below) ────────────────

add_commit \
    "Initial commit" \
    "README.md" \
    "# Test Project

A sample repository for GitGo testing." \
    "2026-01-10T09:00:00"

add_commit \
    "Add main source file" \
    "main.go" \
    'package main

func main() {}' \
    "2026-01-15T14:30:00"

add_commit \
    "Expand README with usage section" \
    "README.md" \
    "# Test Project

A sample repository for GitGo testing.

## Usage

Clone and run." \
    "2026-02-03T11:00:00"

# Push so these three commits are considered upstream (pushed).
git push --set-upstream origin main --quiet
echo "Pushed 3 commits to remote."

# ── Unpushed commits (local only — these are the ones GitGo can edit) ──────────

git config user.name  "Alice Dev"
git config user.email "alice@example.com"

add_commit \
    "Update README: add contributing guide" \
    "README.md" \
    "# Test Project

A sample repository for GitGo testing.

## Usage

Clone and run.

## Contributing

Pull requests welcome." \
    "2026-05-20T10:00:00"

add_commit \
    "Refactor: rename entry point function" \
    "main.go" \
    'package main

func main() {
	run()
}

func run() {}' \
    "2026-05-25T16:45:00"

add_commit \
    "Add configuration file" \
    "config.json" \
    '{
  "version": "0.1.0",
  "debug": false
}' \
    "2026-05-27T09:30:00"

echo ""
echo "Test repository ready."
echo "  Working repo : $TEST_REPO_DIR"
echo "  Bare remote  : $TEST_REMOTE_DIR"
echo ""
echo "Commit log:"
git log --oneline
