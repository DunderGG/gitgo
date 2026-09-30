#!/usr/bin/env bash

set -euo pipefail

SKIP_BUILD=0
RUN_AFTER_BUILD=0
NEW_RUN=0
RUN_TESTS=0
VERBOSE=0
WAILS_ARGS=()

for arg in "$@"; do
    case "$arg" in
        --skip-build)
            SKIP_BUILD=1
            ;;
        --run)
            RUN_AFTER_BUILD=1
            ;;
        --new-run)
            RUN_AFTER_BUILD=1
            NEW_RUN=1
            ;;
        --test)
            RUN_TESTS=1
            ;;
        --verbose)
            VERBOSE=1
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

# Remove what GitGo keeps between runs so it starts like a first run: the
# window size (app/window.go) and the WebView's storage, which holds the
# recent repositories list.
clear_saved_state() {
    if command -v pgrep >/dev/null 2>&1 && pgrep -x gitgo >/dev/null 2>&1; then
        echo "Close GitGo before using --new-run; it saves its window size when it closes." >&2
        exit 1
    fi

    local paths=()
    case "$(uname -s)" in
        Darwin)
            paths=(
                "$HOME/Library/Application Support/gitgo/window.json"
                "$HOME/Library/WebKit/com.wails.GitGo"
                "$HOME/Library/Caches/com.wails.GitGo"
            )
            ;;
        MINGW*|MSYS*|CYGWIN*)
            paths=("$APPDATA/gitgo/window.json" "$APPDATA/gitgo.exe")
            ;;
        *)
            paths=(
                "${XDG_CONFIG_HOME:-$HOME/.config}/gitgo/window.json"
                "${XDG_DATA_HOME:-$HOME/.local/share}/gitgo"
                "${XDG_CACHE_HOME:-$HOME/.cache}/gitgo"
            )
            ;;
    esac

    local path
    for path in "${paths[@]}"; do
        if [[ -e "$path" ]]; then
            rm -rf -- "$path"
            echo "Removed: $path"
        fi
    done
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
    echo "Cannot use --skip-build with --run or --new-run." >&2
    exit 1
fi
if [[ "$SKIP_BUILD" -eq 1 ]] && [[ "$RUN_TESTS" -eq 1 ]]; then
    echo "Cannot use --skip-build and --test together." >&2
    exit 1
fi
if [[ "$RUN_AFTER_BUILD" -eq 1 ]] && [[ "$RUN_TESTS" -eq 1 ]]; then
    echo "Cannot use --test with --run or --new-run." >&2
    exit 1
fi

if [[ "$SKIP_BUILD" -eq 1 ]]; then
    echo "Prerequisite check passed. Build skipped (--skip-build)."
    exit 0
fi

echo "Running: wails build"
if [[ "$VERBOSE" -eq 1 ]]; then
    wails build "${WAILS_ARGS[@]}"
else
    # Filter out Wails' INFO banners and sponsorship notice — they add noise
    # on every build but carry no actionable information.
    # Temporarily disable pipefail so grep exiting 1 (no matching lines to
    # remove) does not abort the script. We check wails' own exit code via
    # PIPESTATUS and re-enable pipefail immediately after.
    set +o pipefail
    wails build "${WAILS_ARGS[@]}" 2>&1 | grep -vE '\bINFO\b|sponsoring the project|leaanthony'
    wails_exit="${PIPESTATUS[0]}"
    set -o pipefail
    if [[ "$wails_exit" -ne 0 ]]; then
        echo "wails build failed with exit code $wails_exit" >&2
        exit "$wails_exit"
    fi
fi

echo "Build completed successfully."

if [[ "$RUN_TESTS" -eq 1 ]]; then
    echo "Running tests..."
    go test -count=1 ./...
    echo "All tests passed."
fi

if [[ "$RUN_AFTER_BUILD" -eq 1 ]]; then
    binary_path="$(dirname "$0")/build/bin/gitgo"
    if [[ ! -f "$binary_path" ]]; then
        # Check for .exe suffix on Windows (Cygwin/MSYS2)
        if [[ -f "${binary_path}.exe" ]]; then
            binary_path="${binary_path}.exe"
        else
            echo "Build output not found at: $binary_path" >&2
            exit 1
        fi
    fi
    if [[ "$NEW_RUN" -eq 1 ]]; then
        echo "Clearing saved state (--new-run)..."
        clear_saved_state
    fi
    echo "Launching: $binary_path"
    "$binary_path" &
fi

echo "Success."
