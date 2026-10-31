# <img width="64" height="64" alt="appicon" src="https://github.com/user-attachments/assets/2fc1f1d4-a0f8-43e0-80d5-0e7c0944d188" /> GitGo

[![Go version](https://img.shields.io/github/go-mod/go-version/DunderGG/gitgo?logo=go&logoColor=white)](go.mod)
[![License: GPL-3.0](https://img.shields.io/github/license/DunderGG/gitgo)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)
[![Wails](https://img.shields.io/badge/Wails-v2-red)](https://wails.io/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![CI](https://img.shields.io/github/actions/workflow/status/DunderGG/gitgo/ci.yml?branch=main&label=CI&logo=githubactions&logoColor=white)](https://github.com/DunderGG/gitgo/actions/workflows/ci.yml)

A cross-platform desktop app for editing local Git history, without memorising CLI commands.

GitGo gives you a clean GUI for the history-editing tasks that are tedious on the command line: fixing a typo in a commit message, correcting a timestamp, or updating author details on commits you haven't pushed yet.

> **Scope:** GitGo only edits **unpushed commits**. Pushed history is shown read-only, so shared history stays safe.

---

## Features

- **Edit commit metadata:** message, author name and email, committer name and email, and author date
- **Edit several commits at once:** select multiple unpushed commits and edit them in one rewrite
- **Search the commit list:** by message, author, hash or date, with `author:`, `since:` and other qualifiers; select the matches to fix them together
- **Commit menu:** right-click a commit to edit it or copy its hash, subject or author, for one commit or the whole selection
- **Preview before applying:** every change is shown old-vs-new in a confirmation dialog
- **Work on any branch:** view and edit other local branches without checking them out
- **Run menu:** ready-made, read-only git commands from the header: the history as text or CSV, or the selected commits as patch files, with Copy and Save
- **In-app help:** a walkthrough of editing and applying changes, every keyboard shortcut and the safety rules, behind the `?` button or `F1`
- **Leaves your work alone:** uncommitted changes (staged or not) and the stash are never touched

The [user guide](docs/USER_GUIDE.md) explains each feature and exactly what it changes, including the [keyboard shortcuts](docs/USER_GUIDE.md#keyboard-shortcuts) and [safety rules](docs/USER_GUIDE.md#safety).

---

## Download

Each [release](https://github.com/DunderGG/gitgo/releases) has packages for every platform:

| Platform | Package |
|---|---|
| Windows (x64) | `GitGo-<version>-windows-amd64-setup.exe` (installer) or `GitGo-<version>-windows-amd64.exe` (portable) |
| macOS (Intel and Apple Silicon) | `GitGo-<version>-macos-universal.dmg` |
| Linux (x64) | `gitgo_<version>_amd64.deb` for Debian 12+ and Ubuntu 22.04+, or `GitGo-<version>-linux-amd64.tar.gz` |

The macOS app is not notarised yet, so macOS blocks the first launch: open it from **System Settings → Privacy & Security → Open Anyway**. On Linux, GitGo needs GTK 3 and WebKitGTK 4.1 (`libwebkit2gtk-4.1-0`); the `.deb` installs them, and the tarball expects them to be installed.

---

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

No `git` binary is needed at runtime; all Git operations use [go-git](https://github.com/go-git/go-git). Only the optional **Run** menu runs `git`, and it is disabled when git is not installed.

### Run in development

```bash
wails dev
```

This starts the Go backend and the Vite dev server together, with hot reload on both sides.

### Build a production binary

```bash
.\scripts\build.ps1   # Windows
./scripts/build.sh    # Linux/macOS
```

Both scripts check prerequisites (`go`, `node`, `npm`, `wails`) and their versions before running `wails build`. The binary ends up in `build/bin/`.

| Flag | Windows | Linux/macOS | Effect |
|---|---|---|---|
| Check only | `-SkipBuild` | `--skip-build` | Run prerequisite checks without building |
| Build and run | `-Run` | `--run` | Launch the app after a successful build |
| Build and run as a new user | `-NewRun` | `--new-run` | Like `-Run`, but first delete GitGo's config folder (settings, recent repositories and window size), so the app starts as on a first run |

`go build` alone is not enough: the frontend must be compiled first and embedded into the Go binary, which `wails build` does for you.

### Try it on a test repository

```bash
.\scripts\setup-testdata.ps1   # Windows
./scripts/setup-testdata.sh    # Linux/macOS
```

This creates `testdata/test-repo` with three pushed and three unpushed commits (one of them SSH-signed) and a bare `testdata/test-remote`. Open `testdata/test-repo` in GitGo to experiment safely. Re-run the script to reset it.

### Run the tests

```bash
go test ./...
```

The tests build real repositories on disk in temporary directories.

### Publish a release

```bash
git tag v1.2.0
git push origin v1.2.0
```

Pushing a version tag runs the [release workflow](.github/workflows/release.yml): it builds the packages above on Windows, macOS and Linux runners, stamps the tag's version into them, and attaches them with a `SHA256SUMS` file to a GitHub Release for the tag. A tag with a suffix, such as `v1.2.0-beta.1`, makes a pre-release. To try the packaging without publishing, run the workflow by hand from the Actions tab; the packages are kept as artifacts of the run.

---

## Tech Stack

- **[Wails v2](https://wails.io/)**: desktop shell that bridges Go and a web frontend into a single native binary per platform
- **Go** with **[go-git v5](https://github.com/go-git/go-git)**: backend and all Git operations (pure Go)
- **React 18 + TypeScript**: frontend UI
- **[Zustand](https://github.com/pmndrs/zustand)**: frontend state management
- **Tailwind CSS v3**: styling, dark theme throughout
- **Vite**: frontend build tool and dev server (managed by Wails)

---

## Project Structure

```
gitgo/
├── main.go               # Wails entry point; embeds frontend/dist into the binary
├── app/                  # IPC layer: methods bound to the frontend, and the DTOs they exchange
├── git/                  # Git logic: open repo, log, unpushed detection, rewrite, undo, reflog, refs, signatures
├── frontend/
│   └── src/
│       ├── App.tsx       # Root layout
│       ├── components/   # CommitList, EditPanel, BulkEditPanel, ConfirmDialog, StatusBar, ...
│       ├── hooks/        # useKeyboardShortcuts
│       └── store/        # Zustand store (repoStore.ts)
├── docs/                 # Architecture, roadmap and PlantUML diagrams
└── scripts/              # Build and test-data setup scripts
```

### Documentation

- [docs/USER_GUIDE.md](docs/USER_GUIDE.md): how each feature works and exactly what it changes
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): design overview and a breakdown of every file
- [docs/ROADMAP.md](docs/ROADMAP.md): completed phases, planned work and future ideas
- [docs/BACKUPS.md](docs/BACKUPS.md): design proposal for saving and restoring backups of a branch's history
- [frontend/FRONTEND_GUIDE.md](frontend/FRONTEND_GUIDE.md): a beginner-friendly walkthrough of the TypeScript/React code
- [scripts/](scripts/): build and test-data setup scripts

---

## Author

**David Bennehag** - [@DunderGG](https://github.com/DunderGG) - [dunder.gg](https://dunder.gg)

---

## License

This project is licensed under the GPL-3.0. See the [LICENSE](LICENSE) file for details.
