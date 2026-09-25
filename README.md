# <img width="64" height="64" alt="appicon" src="https://github.com/user-attachments/assets/2fc1f1d4-a0f8-43e0-80d5-0e7c0944d188" /> GitGo

[![Go version](https://img.shields.io/github/go-mod/go-version/DunderGG/gitgo?logo=go&logoColor=white)](go.mod)
[![License: GPL-3.0](https://img.shields.io/github/license/DunderGG/gitgo)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)
[![Wails](https://img.shields.io/badge/Wails-v2-red)](https://wails.io/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![Last commit](https://img.shields.io/github/last-commit/DunderGG/gitgo)](https://github.com/DunderGG/gitgo/commits/main)

A cross-platform desktop app for editing local Git history, without memorising CLI commands.

GitGo gives you a clean GUI for the history-editing tasks that are tedious on the command line: fixing a typo in a commit message, correcting a timestamp, or updating author details on commits you haven't pushed yet.

> **Scope:** GitGo only edits **unpushed commits**. Pushed history is shown read-only, so shared history stays safe.

---

## Features

- **Edit commit metadata:** message, author name and email, and author date (to the second, in the commit's own time zone), with quick −1d / −1h / +1h / +1d / Now buttons
- **Shift several commits at once:** select multiple unpushed commits and move all their dates in one rewrite
- **Preview before applying:** every change is shown old-vs-new in a confirmation dialog
- **One-step undo:** revert the last rewrite with `Ctrl+Z`
- **Work on any branch:** view and edit other local branches without checking them out
- **In-app help:** a walkthrough of editing and applying changes, every keyboard shortcut and the safety rules, behind the `?` button or `F1`
- **Leaves your work alone:** uncommitted changes (staged or not) and the stash are never touched

### Safety

- Unpushed commits are computed like `git rev-list HEAD ^@{u}`, and commits on any remote-tracking branch are excluded too
- Pushed state is re-checked right before every edit and undo
- Every rewrite and undo is written to the reflog (entries start with `gitgo:`), so you can also recover with `git reset --hard <branch>@{1}`
- Committer identity and extra commit headers (`encoding`, `mergetag`, …) are preserved
- Other local branches pointing at an edited commit are moved along with it; tags are never moved, only flagged
- You are warned before an edit that would drop GPG/SSH signatures

Squash, reorder and drop are planned next, followed by CI and installable binaries. See [docs/ROADMAP.md](docs/ROADMAP.md) for the full roadmap.

---

## Keyboard Shortcuts

| Key | Action |
|---|---|
| `↑` / `↓` | Move the selection between commits |
| `Shift+↑` / `Shift+↓`, `Ctrl`/`Shift`+click | Select several unpushed commits |
| `Enter` | Open the edit panel for the selected commit |
| `Escape` | Close the edit panel or dialog |
| `Ctrl+Z` | Undo the last rewrite |
| `F5` / `Ctrl+R` | Reload the repository from disk |
| `F1` | Open the in-app help (also the `?` button in the header) |

On macOS, `Cmd` works in place of `Ctrl`.

---

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

No `git` binary is needed at runtime; all Git operations use [go-git](https://github.com/go-git/go-git).

### Run in development

```bash
wails dev
```

This starts the Go backend and the Vite dev server together, with hot reload on both sides.

### Build a production binary

```bash
./build.ps1   # Windows
./build.sh    # Linux/macOS
```

Both scripts check prerequisites (`go`, `node`, `npm`, `wails`) and their versions before running `wails build`. The binary ends up in `build/bin/`.

| Flag | Windows | Linux/macOS | Effect |
|---|---|---|---|
| Check only | `-SkipBuild` | `--skip-build` | Run prerequisite checks without building |
| Build and run | `-Run` | `--run` | Launch the app after a successful build |

`go build` alone is not enough: the frontend must be compiled first and embedded into the Go binary, which `wails build` does for you.

### Try it on a test repository

```bash
./setup-testdata.ps1   # Windows
./setup-testdata.sh    # Linux/macOS
```

This creates `testdata/test-repo` with three pushed and three unpushed commits (one of them SSH-signed) and a bare `testdata/test-remote`. Open `testdata/test-repo` in GitGo to experiment safely. Re-run the script to reset it.

### Run the tests

```bash
go test ./...
```

The tests build real repositories on disk in temporary directories.

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
│       ├── components/   # CommitList, EditPanel, BulkDatePanel, ConfirmDialog, StatusBar, ...
│       ├── hooks/        # useKeyboardShortcuts
│       └── store/        # Zustand store (repoStore.ts)
├── docs/                 # Architecture, roadmap and PlantUML diagrams
├── build.ps1, build.sh   # Prerequisite checks + production build
└── setup-testdata.*      # Create a sample repository under testdata/
```

### Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): design overview and a breakdown of every file
- [docs/ROADMAP.md](docs/ROADMAP.md): completed phases, planned work and future ideas
- [frontend/FRONTEND_GUIDE.md](frontend/FRONTEND_GUIDE.md): a beginner-friendly walkthrough of the TypeScript/React code

---

## Author

**David Bennehag** - [@DunderGG](https://github.com/DunderGG) - [dunder.gg](https://dunder.gg)

---

## License

This project is licensed under the GPL-3.0. See the [LICENSE](LICENSE) file for details.
