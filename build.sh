#!/usr/bin/env bash

set -euo pipefail

SKIP_BUILD=0
RUN_AFTER_BUILD=0
WAILS_ARGS=()

for arg in "$@"; do
    case "$arg" in
        --skip-build)
            SKIP_BUILD=1
            ;;
        --run)
            RUN_AFTER_BUILD=1
            ;;
        *)
            WAILS_ARGS+=("$arg")
            ;;
    esac
done

require_command() {
    local name="$1"
    if ! command -v "$name" >/dev/null 2>&1; then
        echo "Missing prerequisite: '$name' is not installed or not on PATH." >&2
        exit 1
    fi
}

assert_go_version() {
    local raw major minor
    raw="$(go version)"

    if [[ "$raw" =~ go([0-9]+)\.([0-9]+) ]]; then
        major="${BASH_REMATCH[1]}"
        minor="${BASH_REMATCH[2]}"
    else
        echo "Could not parse Go version output: $raw" >&2
        exit 1
    fi

    if (( major < 1 || (major == 1 && minor < 25) )); then
        echo "Go 1.25+ is required. Found: $raw" >&2
        exit 1
    fi
}

assert_node_version() {
    local raw major
    raw="$(node --version)"

    if [[ "$raw" =~ ^v([0-9]+) ]]; then
        major="${BASH_REMATCH[1]}"
    else
        echo "Could not parse Node.js version output: $raw" >&2
        exit 1
    fi

    if (( major < 20 )); then
        echo "Node.js 20+ is required. Found: $raw" >&2
        exit 1
    fi
}

echo "Checking prerequisites..."
require_command go
require_command node
require_command npm
require_command wails

assert_go_version
assert_node_version

go_version="$(go version)"
node_version="$(node --version)"
npm_version="$(npm --version)"
wails_version="$(wails version | awk 'NF { print; exit }')"

echo "Go:    $go_version"
echo "Node:  $node_version"
echo "npm:   $npm_version"
echo "Wails: $wails_version"

if [[ "$SKIP_BUILD" -eq 1 ]] && [[ "$RUN_AFTER_BUILD" -eq 1 ]]; then
    echo "Cannot use --skip-build and --run together." >&2
    exit 1
fi

if [[ "$SKIP_BUILD" -eq 1 ]]; then
    echo "Prerequisite check passed. Build skipped (--skip-build)."
    exit 0
fi

echo "Running: wails build"
wails build "${WAILS_ARGS[@]}"

echo "Build completed successfully."

if [[ "$RUN_AFTER_BUILD" -eq 1 ]]; then
    binary_path="$(dirname "$0")/build/bin/gitgo"
    if [[ ! -f "$binary_path" ]]; then
        echo "Build output not found at: $binary_path" >&2
        exit 1
    fi
    echo "Launching: $binary_path"
    "$binary_path" &
fi
