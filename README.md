# <img width="64" height="64" alt="appicon" src="https://github.com/user-attachments/assets/2fc1f1d4-a0f8-43e0-80d5-0e7c0944d188" /> GitGo

A cross-platform desktop application for managing local Git repository history — without memorising CLI commands.

GitGo gives you a clean GUI for the history-editing tasks that are tedious on the command line: fixing a typo in a commit message, correcting a timestamp, or updating author details on commits you haven't pushed yet.

---

## Features

> **Scope:** GitGo only operates on **unpushed commits**. Pushed history is shown read-only and cannot be modified, keeping shared repository history safe.

**Phase 1 (complete)**
- Open any local Git repository
- Browse commit history with a clear pushed / unpushed distinction

**Phase 2 (planned) — Core Editing**
- Edit commit message, date, and author metadata
- Preview every change before it is applied
- Automatic stash/unstash of uncommitted work around history rewrites

**Phase 3 (planned) — UX Polish**
- Recent repositories list
- Undo last rewrite operation
- Branch selector
- Keyboard shortcuts

**Phase 4 (planned) — Advanced Operations**
- Squash, reorder, and drop unpushed commits
- Split commit *(stretch goal)*

**Phase 5 (planned) — Distribution**
- GitHub Actions CI pipeline
- Signed, installable binaries for Windows, macOS, and Linux

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full detail on each phase.

---

## Tech Stack

- **[Wails v2](https://wails.io/)** — desktop shell that bridges Go and a web frontend, producing a single native binary per platform
- **Go 1.25+** — backend; all Git operations via [`go-git/go-git v5`](https://github.com/go-git/go-git) (pure Go, no `git` binary required for core operations)
- **React 18 + TypeScript** — frontend UI
- **Zustand** — frontend state management (no boilerplate, no Provider)
- **Tailwind CSS v3** — utility-first styling; dark theme throughout
- **Vite** — frontend build tool and dev server (managed by Wails)

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a detailed breakdown of each technology and every file in the project.

If you are new to the frontend stack, see [frontend/FRONTEND_GUIDE.md](frontend/FRONTEND_GUIDE.md) for a beginner-friendly walkthrough of the TypeScript/React files.

---

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Run in development

```bash
wails dev
```

This starts the Go backend and the Vite dev server together, with hot reload on both sides.

### Build a production binary

Windows:

```bash
./build.ps1
```

Linux/macOS:

```bash
./build.sh
```

Both scripts check prerequisites (`go`, `node`, `npm`, `wails`) and version requirements (Go 1.25+, Node.js 20+) before running `wails build`.

Optional flags:

| Flag | Windows | Linux/macOS | Effect |
|---|---|---|---|
| Check only | `-SkipBuild` | `--skip-build` | Run prerequisite checks without building |
| Build and run | `-Run` | `--run` | Launch the app automatically after a successful build |

Examples:

```bash
# Windows — check prerequisites only
./build.ps1 -SkipBuild

# Windows — build and immediately launch the app
./build.ps1 -Run
```

```bash
# Linux/macOS — check prerequisites only
./build.sh --skip-build

# Linux/macOS — build and immediately launch the app
./build.sh --run
```

Wails still performs the final binary build. `go build` alone is not enough because the frontend must be compiled first and embedded into the Go binary.

---

## Project Structure

```
GitGo/
├── build.sh              # Prerequisite checks + production build wrapper (Linux/macOS)
├── build.ps1             # Prerequisite checks + production build wrapper (Windows)
├── main.go               # Wails entry point; embeds frontend/dist into the binary
├── app/
│   ├── app.go            # IPC controller; bound methods exposed to the frontend
│   └── models.go         # JSON-serialisable DTOs shared between Go and TypeScript
├── git/
│   ├── repo.go           # Open repo; detect branch, detached HEAD, in-progress ops, unpushed set
│   ├── log.go            # Walk commit log; populate CommitEntry list
│   └── git_test.go       # Unit tests (real on-disk repos via t.TempDir)
├── frontend/
│   ├── src/
│   │   ├── App.tsx       # Root layout; switches between RepoSelector and CommitList
│   │   ├── components/   # RepoSelector, CommitList, StatusBar (+ EditPanel, ConfirmDialog in Phase 2)
│   │   └── store/        # Zustand store (repoStore.ts)
│   └── wailsjs/          # Auto-generated Wails bindings (gitignored)
└── docs/                 # Architecture and roadmap documentation
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a full design overview and [docs/ROADMAP.md](docs/ROADMAP.md) for planned features.

---

## Status

**Phase 1 complete.** The app compiles and runs. Opening a repository displays the full commit log with pushed/unpushed distinction. Phase 2 (commit editing) is next — see [docs/ROADMAP.md](docs/ROADMAP.md).

---

## 👤 Author

**David Bennehag** - [@DunderGG](https://github.com/DunderGG) - [dunder.gg](https://dunder.gg)

---

## License

This project is licensed under the GPL-3.0 - see the [LICENSE](LICENSE) file for details.
