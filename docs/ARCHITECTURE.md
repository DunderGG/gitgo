# GitGo — Architecture

## Overview

GitGo is a cross-platform desktop application for managing local Git repository history. It is built using **Wails v2** (Go backend + React/TypeScript frontend), communicating over Wails' built-in IPC bridge. The application is intentionally scoped to **unpushed commits only**, preventing any accidental rewriting of shared history.

---

## Tech Stack

| Concern | Technology |
|---|---|
| Desktop shell | Wails v2 |
| Backend language | Go 1.25+ |
| Git operations | `go-git/go-git` v5 (pure Go) |
| Frontend language | TypeScript |
| Frontend framework | React 18 |
| Frontend state | Zustand |
| Frontend styling | Tailwind CSS v3 |
| Build / bundle | Vite (via Wails) |

---

### Wails v2

[Wails](https://wails.io) is the desktop shell framework that binds a Go backend to a web-based frontend, producing a single native binary per platform. It is the Go equivalent of Electron — but with no bundled Chromium. Instead, it uses the OS's built-in webview (WebView2 on Windows, WebKit on macOS and Linux), keeping binary sizes small.

**Why Wails over Electron?**  
GitGo's core logic (git operations, safety checks, filesystem access) belongs in Go. Electron would require re-implementing that logic in Node.js or spawning a Go subprocess. With Wails, Go *is* the backend — there is no subprocess boundary, no serialisation overhead for IPC, and no Node.js runtime to ship.

**How the IPC bridge works:**  
At build time, Wails inspects the methods on the bound `App` struct and generates matching TypeScript stubs in `frontend/wailsjs/go/`. Calling one of those stubs from the frontend triggers a call to the corresponding Go method, with arguments and return values automatically serialised through JSON. A returned `error` from Go becomes a rejected `Promise` on the frontend side. This means all Go methods must return `T` or `(T, error)` — no other signatures are valid for binding.

**`wails dev` vs `wails build`:**  
`wails dev` runs the Vite frontend dev server with hot module replacement and simultaneously regenerates the `wailsjs/` bindings whenever a bound Go method signature changes. `wails build` compiles the frontend with Vite, embeds the output into the Go binary, and produces the final platform executable. Regular `go build` cannot be used because it does not run the Vite build step or embed the frontend assets.

---

### Go 1.25+

Go is the backend language. It handles all git operations, safety enforcement, and communication with the Wails runtime. Key reasons for choosing Go:

- **Single binary deployment** — `wails build` produces one executable with the frontend embedded. No installer, no runtime dependencies for end users (beyond the OS webview, which ships with modern Windows and macOS).
- **Strong concurrency model** — the Wails IPC layer may call bound methods concurrently from the frontend. Go's `sync.Mutex` and goroutines make this straightforward to handle correctly.
- **go-git availability** — the best pure-Go git library (`go-git`) exists in the Go ecosystem, avoiding a dependency on a `git` subprocess for core operations.
- **Testability** — the `git/` package has no Wails dependency and can be unit tested with standard `go test`, running real git operations against temporary on-disk repositories.

---

### go-git/go-git v5

[go-git](https://github.com/go-git/go-git) is a pure-Go implementation of git. It parses and writes the git object store (commits, trees, blobs, refs) without shelling out to the `git` binary, giving GitGo full programmatic control over repository operations with no external runtime dependency.

**Why not shell out to `git`?**  
Spawning a `git` subprocess requires `git` to be installed and on the user's `PATH`, introduces parsing fragility (output format changes between versions), and makes the code harder to test in isolation. go-git operations are pure function calls that return typed Go values.

**Known limitations — important for contributors:**

| Operation | go-git support | Workaround |
|---|---|---|
| Cherry-pick | None | Diff-and-apply approach in `git/rewrite.go` (Phase 2) |
| Interactive rebase | None | Programmatic commit-graph walk in `git/rewrite.go` (Phase 2) |
| Stash | None | Not needed: rewrites only change commit metadata, so the working tree and index are never touched (auto-stash was removed in Phase 4) |
| Shallow clones | Limited | Not a concern for GitGo's local-repo use case |

The lack of a cherry-pick API is the most significant constraint. Rewriting a commit that is not at HEAD requires walking every commit between the target and HEAD, recomputing each tree from a diff, writing new commit objects, and resetting the branch ref to the new tip. See the Rewrite Strategy section below for details.

---

### TypeScript

TypeScript is the frontend language. Every file in `frontend/src/` is `.ts` or `.tsx`. Using TypeScript over plain JavaScript means:

- The auto-generated Wails bindings in `wailsjs/go/app/App.d.ts` and `wailsjs/go/models.ts` provide compile-time type safety across the IPC boundary — a mismatch between a Go return type and how the frontend uses it becomes a type error at build time, not a runtime crash.
- The Zustand store state shape is typed, preventing silent property-name typos.
- `tsc --noEmit` can be run in CI to catch type errors independently of the Vite build.

---

### React 18

[React](https://react.dev) is the frontend UI framework. Components are functional and use hooks. React 18 is the minimum version required for the concurrent rendering features Wails v2's frontend scaffold expects.

GitGo's component tree is shallow by design — `App` → (`RepoSelector` | `CommitList`) + `StatusBar`. React's declarative model means view updates are driven entirely by Zustand store changes, with no imperative DOM manipulation.

---

### Zustand

[Zustand](https://github.com/pmndrs/zustand) is the frontend state management library. It was chosen over Redux or React Context for three reasons:

1. **No boilerplate** — state and actions are defined in a single `create()` call in `repoStore.ts`. There is no action/reducer split.
2. **No Provider** — the store is a module-level singleton. Any component can call `useRepoStore(selector)` without being wrapped in a context provider, which simplifies `App.tsx`.
3. **Selector-based subscriptions** — components subscribe to only the slice of state they need (e.g. `useRepoStore(s => s.commits)`), so unrelated state changes do not trigger unnecessary re-renders.

The entire application state lives in one store (`repoStore.ts`). There is no local component state that drives business logic — all IPC results are written to the store, and components read from it reactively.

---

### Tailwind CSS v3

[Tailwind](https://tailwindcss.com) is a utility-first CSS framework. Rather than writing CSS files, styles are composed from atomic class names directly in JSX (e.g. `"flex flex-col h-screen bg-gray-900 text-gray-100"`).

**Why Tailwind?**  
GitGo uses a consistent dark theme throughout. Tailwind's grey and indigo palette provides all the tones needed without any custom CSS. The constraint of utility classes naturally limits visual inconsistency — there is no risk of one component defining `color: #6366f1` while another uses `color: indigo` for what should be the same value.

**Colour palette in use:**
| Role | Classes |
|---|---|
| Page background | `bg-gray-900` |
| Panel / header background | `bg-gray-800` |
| Borders | `border-gray-700` / `border-gray-800` |
| Primary text | `text-gray-100` / `text-gray-200` |
| Muted text | `text-gray-400` / `text-gray-500` |
| Accent (branch, unpushed dot, buttons) | `indigo-400` / `indigo-600` |
| Warning (no remote / no upstream) | `text-yellow-400` |
| Error | `text-red-400` |

All new components must use classes from this palette. Avoid one-off hex colours or inline styles.

---

### Vite

[Vite](https://vite.dev) is the frontend build tool and development server. It handles TypeScript compilation, JSX transformation, Tailwind's PostCSS pass, and module bundling. Wails invokes Vite automatically — `npm run dev` is called by `wails dev`, and `npm run build` is called by `wails build`. The compiled output lands in `frontend/dist/`, which is then embedded into the Go binary.

Vite's key contribution in development is near-instant hot module replacement: editing a `.tsx` file updates the running app in the webview within milliseconds, with no full page reload needed for component changes.

---

## High-Level Layer Diagram

See [diagrams/layer-diagram.puml](diagrams/layer-diagram.puml).

> In the diagram, `git package` is a conceptual label for `git/repo.go` + `git/log.go` (+ the future `git/rewrite.go`). The `App` controller calls functions in those files directly — there is no `GitService` struct.

---

## File-by-File Reference

### Backend

---

#### `main.go`

The Wails entry point. Its only responsibilities are:

1. **Embed the frontend** — the `//go:embed all:frontend/dist` directive bundles the compiled Vite output into the binary at build time so the app ships as a single executable with no external assets.
2. **Configure the window** — title (`"GitGo"`), initial size (1200×800), minimum size (900×600, below which the header and commit columns no longer fit), and background colour (the same `gray-900` used by Tailwind, preventing a flash of white on startup). Platform options keep the native chrome dark to match the dark-only UI: `windows.Dark` theme on Windows, `NSAppearanceNameDarkAqua` plus an About panel (title, description, icon) on macOS, and the window icon and program name on Linux. The icon comes from `build/appicon.png`, embedded with `//go:embed` because Linux and the macOS About panel need it at runtime.
3. **Wire lifecycle hooks** — `OnStartup: app.Startup` passes the Wails context into the `App` struct so bound methods can use it for dialogs and events.
4. **Register bindings** — `Bind: []interface{}{app}` exposes all exported methods on `*App` to the frontend IPC bridge.

Nothing else belongs here. All application logic lives in `app/` and `git/`.

---

#### `app/app.go`

The IPC controller. It holds a single `*App` struct with three fields:

- `ctx context.Context` — the Wails runtime context, stored in `Startup` and used for native dialog calls.
- `mutex sync.Mutex` — guards `repoState` so concurrent IPC calls from the frontend cannot race.
- `repoState *git.RepoState` — the currently open repository; `nil` when no repo is loaded.
- `lastRewrite *rewriteRecord` — the branch ref plus its tip hash before and after the most recent successful rewrite, used by `UndoLastOperation`. In memory only; cleared by `OpenRepository` and after an undo.

**Bound methods** (each maps directly to a callable in `wailsjs/go/app/App.js`):

| Method | What it does |
|---|---|
| `SelectDirectory() (string, error)` | Calls `runtime.OpenDirectoryDialog` with the Wails context to show a native OS folder picker. Returns the selected path or an empty string if the user cancels. Implemented as a bound Go method rather than via the auto-generated runtime JS, because that file is overwritten on every build. |
| `OpenRepository(path string) (RepoInfo, error)` | Calls `git.Open(path)` to validate the path and build a `RepoState`. Stores the state under the mutex for later use. Maps the result to a `RepoInfo` DTO and returns it. Errors from `git.Open` (not a repo, detached HEAD, operation in progress) propagate directly as a rejected Promise on the frontend. |
| `GetCommitLog() ([]CommitSummary, error)` | Reads `repoState` under the mutex; returns an error if no repo is open. Calls `git.Log(state, 0)` to walk up to 100 commits, then maps each `git.CommitEntry` to a `CommitSummary` DTO — including formatting the author date as an RFC 3339 string so the frontend can parse it with `new Date()`. |
| `GetCommitDetail(hash string) (CommitDetail, error)` | Looks up a commit object by hash in the currently opened repository and returns full metadata (message, author name/email, date, unpushed flag) for the edit UI. |
| `RefreshLog() ([]CommitSummary, error)` | Re-opens the current repository path, refreshes `repoState` (including the unpushed set), and returns an updated commit summary list. |
| `UpdateCommit(req EditRequest) (OperationResult, error)` | Applies metadata edits for an unpushed commit via `RebaseRewrite`. Re-reads the repository, performs the server-side unpushed safety check, records the pre/post-rewrite tips in `lastRewrite`, then refreshes in-memory state (shared with `ShiftCommitDates` through `runRewrite`). |
| `ShiftCommitDates(req ShiftRequest) (OperationResult, error)` | Moves the author date (optionally also the committer date) of several unpushed commits by `req.Minutes` in one `ShiftDates` rewrite, keeping each commit's offset. One undo reverts the whole batch. |
| `GetAffectedRefs(hashes []string) ([]AffectedRef, error)` | Lists other branches and tags an edit of the given commits would leave behind, for the confirm dialog. |
| `ReloadRepository() (RepoInfo, error)` | Re-opens the current repository and branch from disk (`git.OpenBranch`) and returns fresh `RepoInfo`, so upstream, remote and checked-out state are re-read too. If the branch no longer exists it falls back to the checked-out branch. Keeps `lastRewrite`; if the branch moved, `UndoLastOperation` reports it. |
| `SwitchBranch(branch string) (RepoInfo, error)` | Calls `git.OpenBranch(path, branch)` to target another local branch **without checking it out**: HEAD and the working tree are untouched, and later edits move only that branch's ref. Replaces `repoState`, clears `lastRewrite`, and returns the new `RepoInfo`. |
| `ListBranches() ([]string, error)` | Returns the short names of all local branches, sorted alphabetically. |
| `UndoLastOperation() (OperationResult, error)` | Re-opens the repo and calls `git.ResetBranch` to move the branch from the post-rewrite tip back to the pre-rewrite tip. Only one level of undo is kept. Returns `ErrBranchMoved` (and drops the record) if the branch no longer points at the rewritten tip, e.g. a new commit was made. The worktree is not touched: rewrites only change metadata, so both tips have the same tree. |
| `CanUndo() bool` | Reports whether `lastRewrite` is set. Used by the frontend to re-sync the Undo button after a failed undo. |

The app layer owns all DTO mapping (Go types ↔ JSON-serialisable structs). The `git/` package knows nothing about the `app/models.go` types.

---

#### `app/models.go`

Data transfer types that cross the IPC boundary. All fields carry `json:` tags so Wails serialises them correctly into TypeScript. The types are also mirrored in `wailsjs/go/models.ts` (auto-generated).

| Type | Purpose |
|---|---|
| `RepoInfo` | Returned by `OpenRepository` and `SwitchBranch`. Carries the absolute repo path, the selected branch name, `IsCheckedOut` (whether that branch is the one HEAD points at), and two boolean flags: `HasRemote` (at least one remote configured) and `HasUpstream` (current branch tracks a remote branch). The frontend uses these flags to decide what to display in `StatusBar`. |
| `CommitSummary` | One row in the commit list. Contains the full 40-character hash, a 7-character short hash for display, the first line of the commit message, the author name, an RFC 3339 date string, and `IsUnpushed` — the flag the frontend uses for visual distinction and to gate editing. |
| `CommitDetail` | *(Phase 2)* Full commit metadata for the edit form. Extends `CommitSummary` with `AuthorEmail` so the user can edit it. |
| `EditRequest` | *(Phase 2)* The payload the frontend sends when the user confirms an edit. Contains the target hash plus all four editable fields: message, author name, author email, and date. |
| `OperationResult` | *(Phase 2)* Returned by all mutating bound methods. `Success bool` lets the frontend distinguish a handled error from an unexpected one; `Message string` provides human-readable context to display. |

---

#### `git/repo.go`

The repository-opening layer. Exposes two sentinel errors and one public function:

**`ErrDetachedHead`** — returned when `HEAD` is not a symbolic reference to a branch (i.e. the repo is in detached HEAD state). GitGo cannot safely scope edits to "unpushed commits" without a branch to anchor to.

**`ErrOperationInProgress`** — returned when git has left a sentinel file or directory indicating an unfinished operation. Editing history during an in-progress merge or rebase would corrupt the repository.

**`ErrNoCommits`** — returned when HEAD points at a branch with no commits yet (a fresh `git init`), instead of go-git's raw "reference not found" error.

**`RepoState`** — the struct threaded through every subsequent git operation. It holds:
- The raw `*gogit.Repository` handle for all go-git calls.
- The resolved working-tree root path (go-git walks up from a subdirectory if needed; this captures the real root).
- The target branch name, `IsCheckedOut` (whether HEAD points at it), and remote/upstream flags. Log, rewrite and undo all read this branch's ref (via the internal `branchTip` helper), not HEAD, so a branch can be viewed and edited without checking it out.
- `UnpushedHashes map[plumbing.Hash]bool` — a set built at open time and reused by both the log layer and (in Phase 2) the rewrite layer to gate edits.

**`Open(path string) (*RepoState, error)`** — opens the checked-out branch; shorthand for `OpenBranch(path, "")`.

**`ListBranches(state) ([]string, error)`** — short names of all local branches, sorted.

**`OpenBranch(path, branch string) (*RepoState, error)`** — the single entry point. An empty `branch` selects the checked-out branch; an unknown branch returns `ErrBranchNotFound`. Its internal steps in order:

1. `gogit.PlainOpenWithOptions` with `DetectDotGit: true` — accepts a path to any subdirectory within a repo, not just the root.
2. Resolve the true working-tree root via `worktree.Filesystem.Root()`.
3. `detectInProgressOperation` — checks for `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `BISECT_LOG`, `rebase-merge/`, and `rebase-apply/` in `.git/`. Returns `ErrOperationInProgress` if any exist.
4. Read `HEAD`; reject non-branch refs with `ErrDetachedHead`. Then resolve the target branch ref (HEAD's branch when `branch` is empty).
5. `resolveUpstream` — reads the repo's git config, finds the branch's `[branch "name"] remote` and `merge` entries, constructs the `refs/remotes/<remote>/<branch>` ref name, and resolves it to a hash. If no remote or no upstream is configured, returns zero-hash and sets the flags accordingly.
6. `computeUnpushed` — walks the commit log from the target branch tip; stops when it reaches the upstream tip hash. Every commit encountered before that point is added to the `UnpushedHashes` set. If there is no upstream, all commits in the log are treated as unpushed.

---

#### `git/log.go`

The commit-log layer. Exposes one public type and one public function:

**`CommitEntry`** — the git package's own representation of a commit row. Uses `plumbing.Hash` (a `[20]byte`) for the hash rather than a string, keeping the type system honest at the git layer. The app layer converts this to `app.CommitSummary` with string hashes for JSON serialisation.

**`Log(state *RepoState, limit int) ([]CommitEntry, error)`** — walks the commit graph from the tip of `state.Branch` using go-git's `repo.Log`. If `limit` is 0 or negative, the default of 100 is used. For each commit:
- Short hash is the first 7 characters of the 40-character hex string.
- Message is the first line only (via `firstLine`) — multi-line commit messages show only the subject in the list.
- `IsUnpushed` is a direct lookup into `state.UnpushedHashes` — O(1) per commit.
- `Date` is `commit.Author.When`, which reflects the author timestamp (not the committer timestamp), consistent with what `git log` shows by default.

---

#### `git/git_test.go`

Unit tests for the `git` package. All tests use real on-disk repositories created in `t.TempDir()` directories and run actual `git` commands via `os/exec` to set up state. go-git's in-memory storage is not used — it lacks support for operations that Phase 2 will need.

Tests are grouped into two areas:

**`Open` tests:**
- `TestOpen_ValidRepo` — happy path; checks branch name and path.
- `TestOpen_NonRepo` — empty directory returns an error.
- `TestOpen_NoCommits` — freshly initialised repo with no commits; verifies `ErrNoCommits`.
- `TestOpen_DetachedHead` — checks out a commit hash directly; verifies `ErrDetachedHead`.
- `TestOpen_MergeInProgress` — writes a fake `MERGE_HEAD`; verifies `ErrOperationInProgress`.
- `TestOpen_RebaseInProgress` — creates a fake `rebase-merge/` directory; verifies `ErrOperationInProgress`.
- `TestOpen_NoRemote` — no remote configured; all commits should be in `UnpushedHashes`.
- `TestOpen_WithUpstream` — pushes one commit to a bare remote, adds two more locally; verifies exactly 2 unpushed.

**`Log` tests:**
- `TestLog_ReturnsEntries` — 3 commits → 3 entries.
- `TestLog_RespectsDepthLimit` — 5 commits with limit 3 → 3 entries.
- `TestLog_ShortHashLength` — verifies `ShortHash` is exactly 7 characters.
- `TestLog_IsUnpushedFlag` — 1 pushed + 1 unpushed; verifies `IsUnpushed` is correct on each.
- `TestLog_DatePopulated` — verifies `Date` is non-zero and matches the fixed `GIT_AUTHOR_DATE` env var used in the test harness.
- `TestLog_MessageIsFirstLine` — multi-line commit message; verifies only the subject line is returned.
- `TestLog_CommitOrder` — 3 commits; verifies entries are returned newest-first.

---

### Frontend

---

#### `frontend/src/main.tsx`

The React entry point. Mounts the `<App>` component, wrapped in `<ErrorBoundary>`, into `#root` inside `index.html`. It also registers a window `unhandledrejection` listener that passes any uncaught promise rejection (e.g. a failed backend call nobody caught) to `setError`, so it shows in `StatusBar` instead of being lost. No other application logic lives here.

---

#### `frontend/src/errors.ts`

Turns raw error text into messages a user can act on. Wails rejects a bound method's promise with the Go error's message, so matching is done on that text.

- `friendlyError(raw)` — checks an ordered list of regex rules (not a git repository, branch not found, no repository open, commit not found, reflog write failures, invalid date) and returns the first matching friendly message. Unmatched text is tidied: a leading `Error: ` is dropped and the first letter capitalised. The sentinel errors in `git/errors.go` are already written for users and pass through this way.
- `errorText(error)` — converts any thrown value to its message text.

When adding a new backend error whose raw text is not user-friendly, add a rule here.

---

#### `frontend/src/components/ErrorBoundary.tsx`

A class component (React has no hook equivalent) that catches errors thrown while rendering. Instead of a blank window it shows a "Something went wrong" screen explaining that the repository was not changed, a collapsible "Technical details" block with the stack, and three recovery actions: **Try again** (re-render with the same state), **Close repository** (`clearRepo()` and back to `RepoSelector`), and **Reload GitGo** (`window.location.reload()`; the Go backend keeps running). Errors in event handlers and promises are not caught by React boundaries; those go through `setError`.

---

#### `frontend/src/App.tsx`

The root layout component. Renders a full-height flex column with three vertical sections:

- **Header** (fixed height) — application title; when a repo is open, shows the full repository path truncated with `overflow-hidden`, and on the right a `<BranchSelector>` and a **↻ reload** button that calls the store's `reloadRepository` (shows a spinner while it runs, disabled during any git operation).
- **Main** (flex-1, scrollable) — conditionally renders either `<RepoSelector>` (no repo open) or a two-column repo workspace (`<CommitList>` + `<EditPanel>`), driven by `repoInfo` from the Zustand store.
- **Footer** — always-visible `<StatusBar>`.

`App.tsx` owns the top-level conditional render. It subscribes to only `repoInfo` from the store to decide which view to show, keeping re-renders minimal.

It also keeps the window title in sync with the open repository and branch (`"GitGo — <repo folder> (<branch>)"`, or just `"GitGo"` when nothing is open), via the Wails runtime's `WindowSetTitle` plus `document.title`. The runtime call is wrapped in `try`/`catch` because it does not exist when the frontend runs in a plain browser.

---

#### `frontend/src/components/BranchSelector.tsx`

A dropdown in the header listing local branches from `ListBranches()`. The list is loaded when a repository is opened and reloaded whenever the dropdown gains focus, so branches created outside the app show up.

Choosing a branch calls `SwitchBranch(name)` then `GetCommitLog()` and writes both into the store with `setRepo`. The branch is **not** checked out: the log and edits target that branch's ref, and `StatusBar` shows a "Not checked out" notice while `repoInfo.isCheckedOut` is false. The switch runs inside `runGitOperation`: the dropdown shows the chosen branch with a spinner beside it and is disabled while any git operation is running.

---

#### `frontend/src/components/RepoSelector.tsx`

The empty-state view shown before any repository is loaded. Contains a centred card with a heading, subtext, a one-line explanation of what GitGo does (edit unpushed commits; pushed commits are read-only), and an "Open Repository" button.

It also renders a "Recent Repositories" list driven by `repoStore.recentRepos` (persisted in `localStorage`). Each entry can be quick-opened, and entries can be removed manually. When the list is empty, a hint says opened repositories will appear there.

When the button is clicked, `handleOpen` runs the following sequence over the Wails IPC bridge:

1. `SelectDirectory()` — opens the native OS folder picker. Returns empty string if cancelled; bails out immediately.
2. `runGitOperation('Opening repository…', …)` wraps steps 3–5: the status bar shows a spinner with that label, the Open and recent-repository buttons are disabled, and the clicked button shows its own spinner.
3. `OpenRepository(path)` — validates the path on the Go side, builds `RepoState`, and returns `RepoInfo`.
4. `GetCommitLog()` — walks the commit log and returns `[]CommitSummary`.
5. `setRepo(repoInfo, commits)` — writes both into the Zustand store atomically. This triggers the `App.tsx` conditional to switch from `RepoSelector` to `CommitList`.

Any error at any step calls `setError(String(err))`, which maps it to a friendly message that the `StatusBar` renders in red. If a quick-opened recent path appears to no longer exist, `removeRecentRepo(path)` prunes the stale entry from the list.

---

#### `frontend/src/components/CommitList.tsx`

The main view after a repository is opened. Reads `commits` from the Zustand store (reactive — updates automatically if the store changes).

Structure:
- **Column headers** — a fixed header row with labels for hash, message, author, and date.
- **Legend** — a summary row showing the count of unpushed (indigo dot) vs pushed (grey dot) commits.
- **Notice banner** (optional, from `listNotice`) — explains a surprising pushed/unpushed split. At most one shows, in this priority order: no remote (info: every commit counts as unpushed), no upstream (warning: every commit counts as unpushed, avoid editing commits pushed under another name), or no unpushed commits (muted: nothing to edit).
- **Empty branch** — when `commits` is empty, the list is replaced by "This branch has no commits yet."
- **Scrollable commit rows** — each rendered by the internal `CommitRow` component.

`CommitRow` renders one commit. Visual treatment:
- Unpushed commits render at full opacity with an indigo dot. Pushed commits render at 60% opacity with a grey dot, communicating they are read-only.
- Rows are selectable by click. The selected row is highlighted with a darker indigo background and left border. Each row carries a `data-commit-hash` attribute so keyboard code can focus it.
- Keyboard: `Enter` selects the focused row and calls `requestEditFocus()` so `EditPanel` focuses its message field once the commit has loaded (editable commits only). `↑` / `↓` move the selection and focus to the previous / next row.
- The 7-character short hash is monospaced and `select-all` so users can copy it.
- The commit message truncates with Tailwind `truncate` and the full message is in a `title` attribute on hover.
- The date is localised via `toLocaleDateString` rather than shown as a raw ISO string.
- The author column is hidden on narrow viewports (`hidden sm:block`).

---

#### `frontend/src/components/EditPanel.tsx`

The edit workspace for the currently selected commit. Reads `selectedHash` from the Zustand store and lazily fetches full commit metadata with `GetCommitDetail(hash)` whenever the selection changes.

Behaviour:
- With nothing selected, shows a hint: how to select a commit (click, or `↑` / `↓` and `Enter`), or, when the branch has no unpushed commits, that there is nothing to edit.
- Maintains local form state for message, date/time, author name, and author email so the user can edit fields without mutating shared store state on every keystroke.
- Tracks the originally loaded values separately from the current form values so it can detect changes, support reset, and feed the confirmation dialog with an explicit before/after comparison.
- Disables all editable controls while commit details are loading (shown with a spinner), while a rewrite is being submitted, and for pushed commits. `isUnpushed` is read from the matching entry in the store's `commits` (not from the loaded detail), so a reload that finds the commit was pushed makes it read-only immediately without resetting the form. "Review Changes" is also disabled while any other git operation is running.
- The rewrite (`UpdateCommit` + `RefreshLog`) runs inside `runGitOperation('Rewriting commit history…', …)`; the dialog's Apply button shows a spinner meanwhile.
- On submit, opens `<ConfirmDialog>` instead of immediately rewriting history.
- On confirm, calls `UpdateCommit` followed by `RefreshLog`, then writes the refreshed commit list back into the store and clears selection via `setRepo`.
- Consumes the store's `pendingEditFocus` request (set by `Enter` in `CommitList`): once the selected commit has finished loading it focuses the message field if the commit is editable, and clears the request either way. It tracks the hash whose load finished (`loadedHash`) so it never acts on the previous commit's state right after the selection changes.

The panel intentionally owns transient UI state (loading, local form values, dialog visibility, submit-in-progress) while long-lived application state remains in `repoStore.ts`.

---

#### `frontend/src/components/ConfirmDialog.tsx`

A modal confirmation dialog rendered by `EditPanel`. Its only responsibility is review and confirmation — it does not own any repository state itself.

Behaviour:
- Receives precomputed `before` and `after` values from `EditPanel`.
- Shows a side-by-side comparison for message, date/time, author name, and author email.
- Highlights changed values visually so the user can quickly verify what will be rewritten.
- Exposes `Cancel` and `Apply` actions; `Apply` is disabled while a rewrite request is already in flight.
- While open, a capture-phase `keydown` listener on `window` makes `Escape` cancel the dialog (unless a rewrite is in flight) and swallows `Ctrl+Z`, stopping both from reaching the app-wide shortcuts.

---

#### `frontend/src/components/StatusBar.tsx`

A persistent footer bar rendered on every screen. Reads three independent slices from the Zustand store:

- **Left side** — when a repo is open: shows the branch name in indigo, plus a "Not checked out" notice when viewing a branch other than HEAD's. Conditionally appends a yellow "No remote configured" or "No upstream set" notice, driven by `repoInfo.hasRemote` and `repoInfo.hasUpstream`.
- **Right side** — mutually exclusive: while `activity` is set, shows a spinner with the running operation's label; otherwise, if `error` is non-null, shows it in red; otherwise shows the `status` string in muted grey. This means any error immediately replaces a previous status message. Errors are truncated to fit; the tooltip shows the full message plus the raw `errorDetail`, and a **×** button dismisses the error.

Successful rewrite messages from `UpdateCommit` also surface here, including the warning returned when some of the other branches selected in `ConfirmDialog` could not be moved.

When `canUndo` is true, an **Undo** button appears next to the status text. It calls the store's `undoLastOperation` action, the same one the `Ctrl+Z` shortcut uses, and is disabled while any git operation is running (`activity` is set).

---

#### `frontend/src/components/Spinner.tsx`

A small inline SVG spinner (Tailwind `animate-spin`) that inherits the surrounding text colour. Used by `StatusBar`, `RepoSelector`, `BranchSelector`, `EditPanel`, and `ConfirmDialog`.

---

#### `frontend/src/hooks/useKeyboardShortcuts.ts`

Registers the app-wide keyboard shortcuts on `window`; called once from `App.tsx`.

| Shortcut | Effect |
|---|---|
| `F5` / `Ctrl+R` / `Cmd+R` | Calls `reloadRepository()` when a repo is open. Always intercepted, because in the Wails webview these keys would otherwise reload the page and lose all UI state. |
| `Ctrl+Z` / `Cmd+Z` | Calls `undoLastOperation()`. Ignored when the key event comes from an `input`, `textarea`, `select` or editable element, so the browser's own text undo keeps working there. |
| `Escape` | When a commit is selected: clears the selection (closing the edit panel and discarding unsaved form changes) and moves focus back to that row in `CommitList`. |

Row-level keys (`Enter`, `↑`, `↓`) are handled in `CommitList`, and `ConfirmDialog` takes over `Escape` while it is open. The file also exports `focusCommitRow(hash)`, used by both.

The component subscribes to three separate store selectors rather than the whole store, so it only re-renders when one of those three values changes.

---

#### `frontend/src/store/repoStore.ts`

The single source of truth for all application state. Built with Zustand (no Provider needed — the store is module-level). Exports the `useRepoStore` hook and the `RepoInfo` and `CommitSummary` TypeScript interfaces (mirroring `app/models.go`).

**State shape:**

| Field | Type | Description |
|---|---|---|
| `repoInfo` | `RepoInfo \| null` | `null` means no repo is open. Non-null switches the main view from `RepoSelector` to `CommitList`. |
| `commits` | `CommitSummary[]` | The current log. Empty array while no repo is open. |
| `recentRepos` | `string[]` | Most-recent repository paths, stored in `localStorage`, deduplicated, and capped to 10 entries. |
| `selectedHash` | `string \| null` | Currently selected commit hash in `CommitList`. `null` means no row is selected yet. |
| `canUndo` | `boolean` | `true` after a successful rewrite; shows the Undo button in `StatusBar`. Reset by every `setRepo` call and by `clearRepo`. |
| `activity` | `string \| null` | Label of the git operation currently running (e.g. `"Switching to main…"`), or `null` when idle. Drives the status-bar spinner and disables controls that would start another git operation. |
| `pendingEditFocus` | `boolean` | Set by `Enter` on a commit row; consumed by `EditPanel` after the commit loads. |
| `status` | `string` | Most-recent informational message (e.g. `"Opened: /path/to/repo"`). |
| `error` | `string \| null` | Most-recent error, as a user-friendly message (see `errors.ts`). Non-null causes `StatusBar` to show it in red. Setting a new error does not clear `repoInfo` — the repo remains open. |
| `errorDetail` | `string \| null` | The raw error text behind `error`, shown in the `StatusBar` tooltip. `null` when the friendly message says the same thing. |

**Actions:**

| Action | Effect |
|---|---|
| `setRepo(info, commits)` | Sets `repoInfo` and `commits` together, clears `selectedHash` and `error`, sets `status` to `"Opened: <path>"`. |
| `removeRecentRepo(path)` | Removes one path from the recent list and persists the updated list to `localStorage`. |
| `selectCommit(hash)` | Sets `selectedHash` when a commit row is clicked or keyboard-selected. |
| `setCanUndo(canUndo)` | Sets `canUndo`. `EditPanel` sets it to `true` after a rewrite. |
| `reloadRepository()` | Async. Runs as `runGitOperation(reloadActivityLabel, …)`: calls `ReloadRepository` then `GetCommitLog` and writes both into the store directly (not via `setRepo`), keeping the selected commit if it still exists and keeping `canUndo` unless the branch changed. Sets `status` to `"Reloaded from disk"`, or explains the fallback when the branch was deleted. |
| `undoLastOperation()` | Async. No-op unless `canUndo` and not already undoing. Calls `UndoLastOperation` then `RefreshLog` and writes the log back via `setRepo` (which clears `canUndo`). On failure, shows the error and asks `CanUndo()` whether undo is still possible. |
| `requestEditFocus()` / `consumeEditFocus()` | Set / clear `pendingEditFocus`. |
| `setStatus(message)` | Updates `status` without touching anything else. |
| `runGitOperation(label, operation)` | Async. Runs one git operation at a time: sets `activity` to `label`, awaits `operation`, and clears `activity` afterwards. If another operation is already running it skips the new one and returns `undefined`. Errors propagate to the caller. Used for opening a repository, switching branch, rewriting, and undoing. |
| `setError(error)` | Takes raw error text, stores `friendlyError(error)` in `error` and the raw text in `errorDetail`. Pass `null` to dismiss. |
| `clearRepo()` | Resets all state (including `selectedHash`) to initial values — returns the app to the `RepoSelector` view. |

---

### Configuration & Build Files

---

#### `wails.json`

Wails v2 project configuration. Specifies the frontend build commands (`npm run build` for production, `npm run dev` for the hot-reload dev server), the output binary name, and the frontend source directory. Wails reads this to know how to build and embed the frontend.

#### `go.mod` / `go.sum`

Go module definition. Direct dependencies: `github.com/wailsapp/wails/v2` and `github.com/go-git/go-git/v5`. All other entries are transitive. The `go.sum` file locks exact checksums for reproducible builds.

#### `frontend/package.json`

npm package manifest for the frontend. Key dependencies: `react`, `react-dom`, `zustand`. Dev dependencies: `vite`, `tailwindcss`, `typescript`, `@vitejs/plugin-react`.

#### `frontend/vite.config.ts`

Configures Vite to use the React plugin. In `wails dev` mode Wails injects a proxy so the frontend dev server talks to the Go backend.

#### `frontend/tailwind.config.ts`

Configures Tailwind to scan `src/**/*.{ts,tsx}` for class names. The dark theme used throughout the app is built entirely from Tailwind utility classes — no custom CSS.

#### `build/appicon.png` and `build/windows/icon.ico`

The application icon. `appicon.png` is the source image: square (1224×1224) with a transparent background, as Wails expects when generating the macOS `.icns`. It is also embedded into the binary for the Linux window icon and the macOS About panel. `icon.ico` is the Windows icon, with 16–256 px sizes. Wails only regenerates `icon.ico` from `appicon.png` when the `.ico` is missing, so after changing the artwork, delete `icon.ico` and run `wails build` to regenerate it.

#### `build/windows/info.json` and `build/windows/wails.exe.manifest`

Windows-specific resource metadata (version info, UAC manifest). Embedded into the `.exe` by Wails at build time. These are source files and must be committed — only `build/bin/` (the compiled output) is gitignored.

---

## Component Breakdown (Summary)

### Frontend

| Component | Responsibility |
|---|---|
| `App.tsx` | Root layout; switches between `RepoSelector` and the repo workspace (`CommitList` + `EditPanel`) based on store state |
| `RepoSelector` | Empty-state view; orchestrates `SelectDirectory` → `OpenRepository` → `GetCommitLog` → `setRepo`; also renders and manages quick-open recent repositories |
| `BranchSelector` | *(Phase 3)* Header dropdown of local branches; switches the viewed/edited branch via `SwitchBranch` without checking it out |
| `CommitList` | Scrollable commit log; indigo/grey dot for unpushed/pushed; column headers and legend; row selection state; `Enter` / arrow-key navigation |
| `useKeyboardShortcuts` | *(Phase 3)* App-wide `Ctrl+Z` (undo) and `Escape` (close edit panel) shortcuts |
| `StatusBar` | Persistent footer; branch name, remote notices, status/error display including rewrite messages, Undo button after a rewrite |
| `EditPanel` | *(Phase 2)* Edit form for message, date, author; loads commit detail; opens confirm dialog; applies rewrites |
| `ConfirmDialog` | *(Phase 2)* Side-by-side old/new diff before confirming a rewrite |
| `repoStore.ts` | Zustand store; single source of truth for `repoInfo`, `commits`, `recentRepos`, `selectedHash`, `canUndo`, `activity`, `pendingEditFocus`, `status`, `error`; also owns the shared `runGitOperation` and `undoLastOperation` actions |
| `Spinner` | *(Phase 3)* Inline loading indicator used wherever a git operation is in progress |
| `ErrorBoundary` | *(Phase 3)* Catches render errors and shows a recovery screen (try again / close repository / reload) |
| `errors.ts` | *(Phase 3)* Maps raw backend error text to user-friendly messages |

### Backend

| File | Responsibility |
|---|---|
| `main.go` | Wails entry point; embeds frontend, configures window, registers bindings |
| `app/app.go` | IPC controller; bound methods: `SelectDirectory`, `OpenRepository`, `GetCommitLog`, `GetCommitDetail`, `RefreshLog`, `UpdateCommit`, `ShiftCommitDates`, `GetAffectedRefs`, `ReloadRepository`, `SwitchBranch`, `ListBranches`, `UndoLastOperation`, `CanUndo` |
| `app/models.go` | JSON-serialisable DTOs shared between Go and TypeScript |
| `git/repo.go` | `Open` / `OpenBranch`: validate path, detect edge cases, build `RepoState` for a branch with its unpushed set; `ListBranches` |
| `git/log.go` | `Log`: walk commit graph, populate `[]CommitEntry`, respect depth limit |
| `git/git_test.go` | 14 unit tests covering `Open` and `Log` using real on-disk repos |
| `git/rewrite.go` | *(Phase 2)* `RewriteCommits` (edits any set of unpushed commits in one first-parent chain rebuild, one branch move and one reflog entry), with `AmendCommit` / `RebaseRewrite` as single-commit wrappers and `ShiftDates` for bulk date shifts; commit rebuilding (`rebuildCommit`, which keeps encoding and extra headers but drops signatures) and author validation (`validateIdentity`) |
| `git/undo.go` | *(Phase 3)* `ResetBranch`: compare-and-swap the branch ref back to its pre-rewrite tip |
| `git/branch_test.go` | *(Phase 3)* Tests for `ListBranches`, `OpenBranch`, and rewriting/undoing on a branch that is not checked out |

---

## Project Directory Structure

```
GitGo/
├── main.go                  # Wails entry point
├── go.mod
├── go.sum
├── wails.json               # Wails project config
│
├── app/
│   ├── app.go               # App struct — bound methods exposed to frontend
│   └── models.go            # DTOs shared across layers
│
├── git/
│   ├── repo.go              # Open, branch info, in-progress detection, unpushed set
│   ├── log.go               # Commit log walking
│   ├── git_test.go          # Unit tests (real on-disk repos via t.TempDir)
│   ├── rewrite.go           # (Phase 2) Amend + rebase-based rewriting
│   └── undo.go              # (Phase 3) Reset branch to pre-rewrite tip
│
├── frontend/
│   ├── index.html
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   └── src/
│       ├── main.tsx
│       ├── errors.ts               # (Phase 3) Friendly error messages
│       ├── App.tsx
│       ├── components/
│       │   ├── BranchSelector.tsx  # (Phase 3)
│       │   ├── RepoSelector.tsx
│       │   ├── Spinner.tsx         # (Phase 3)
│       │   ├── CommitList.tsx
│       │   ├── EditPanel.tsx       # (Phase 2)
│       │   ├── ErrorBoundary.tsx   # (Phase 3)
│       │   ├── ConfirmDialog.tsx   # (Phase 2)
│       │   └── StatusBar.tsx
│       ├── hooks/
│       │   └── useKeyboardShortcuts.ts  # (Phase 3)
│       ├── store/
│       │   └── repoStore.ts        # Zustand store
│       └── wailsjs/                # Auto-generated by Wails (gitignored)
│           ├── go/app/App.js
│           ├── go/app/App.d.ts
│           ├── go/models.ts
│           └── runtime/            # Wails runtime bindings
│
└── docs/
    ├── ARCHITECTURE.md       # This file
    ├── ROADMAP.md
    └── diagrams/
        ├── layer-diagram.puml
        ├── sequence-open-repo.puml
        └── sequence-edit-commit.puml
```

---

## Data Flow — Component Sequence

### Opening a Repository

See [diagrams/sequence-open-repo.puml](diagrams/sequence-open-repo.puml).

### Editing a Commit

See [diagrams/sequence-edit-commit.puml](diagrams/sequence-edit-commit.puml).

---

## Key Design Decisions

### Safety First
GitGo never exposes operations that can rewrite pushed history. The UI marks pushed commits as read-only, and the backend enforces the same boundary independently — `UpdateCommit` re-derives the unpushed set at call time and rejects any hash not in it. This check cannot be bypassed by the frontend.

### Go-Native Backend
All git logic lives in the Go layer. The frontend is strictly a display and input surface — it holds no git state and makes no git decisions. This keeps the backend fully testable in isolation and ensures the safety boundary cannot be accidentally eroded by frontend changes.

### Rewrite Strategy
- **Last commit only:** `git commit --amend` equivalent via `go-git` Worktree — replace the HEAD commit object with a new one carrying the updated metadata.
- **Older commits:** A programmatic rebase walking from the target commit to `HEAD`:
  1. For each commit in the chain, compute the diff between the commit and its parent using go-git's `object.DiffTree`.
  2. Apply that patch to the accumulated tree, building a new tree object.
  3. Write a new commit object with the updated tree (substituting edited metadata at the target position).
  4. Reset the branch ref to the new tip commit.

> **go-git limitation:** go-git v5 has no native `CherryPick()` or interactive rebase API. The diff-and-apply approach above is the required implementation path. Conflicts during patch application must be detected and reported to the user as an `OperationResult` error — the original ref must be restored on failure.

### No Force Push
GitGo only modifies the local working repository. It exposes no push functionality, ensuring nothing changed by the app can affect a remote without a deliberate separate action by the user.

### Working Tree Is Never Touched
Rewrites only change commit metadata (message, author, dates); every rebuilt commit keeps its original file tree. Moving the branch ref therefore leaves the index and working tree consistent, so uncommitted work — staged or not — and the stash are never touched, whether or not the branch is checked out.

> **History:** earlier versions auto-stashed a dirty worktree with the native `git stash` / `git stash pop`. That was removed: go-git's `Worktree.Status()` ignores `core.fileMode=false`, so on Windows a clean worktree with an executable file looked dirty, the no-op `git stash` was followed by a `git stash pop` that applied and dropped an unrelated stash, and `pop` without `--index` also unstaged staged changes. The native `git` binary is no longer a runtime dependency.

### Wails Binding Constraints
All Go methods on the `App` struct that are exposed to the frontend via `wails.Bind` must conform to one of these return signatures:
- `func (...) T` — returns a single value
- `func (...) (T, error)` — returns a value and an error

The `error` return is automatically serialised into a rejected JavaScript Promise. Every bound method in `app/app.go` must follow this pattern. Methods that only signal success/failure should return `(OperationResult, error)`.

On the frontend, rejected promises are caught and passed to `setError`, which translates the raw text via `friendlyError` in `frontend/src/errors.ts`. Prefer writing Go error messages that are readable as-is (as the sentinels in `git/errors.go` are); otherwise add a mapping rule there.

### Edge Cases & Boundary Conditions
The following conditions must be detected at the start of any operation and returned as errors before any rewrite is attempted:

| Condition | Detection | Behaviour |
|---|---|---|
| No remote configured | `git.Repository.Remotes()` returns empty | Mark all commits as unpushed; disable upstream-based detection; explain it in a banner above `CommitList` |
| Branch has no upstream set | `git.Branch.Remote` is empty | Same as above; surface a notice in `StatusBar` and a warning banner above `CommitList` |
| No unpushed commits | Every commit in the log has `isUnpushed == false` | Banner above `CommitList` and a hint in `EditPanel` that there is nothing to edit |
| Repository has no commits | `git.Repository.Head()` returns `plumbing.ErrReferenceNotFound` | Return `ErrNoCommits`; show the error and stay on `RepoSelector` |
| Detached HEAD state | `git.Repository.Head()` returns a non-branch ref | Show error; disable all editing |
| In-progress git operation | Check for `.git/MERGE_HEAD`, `.git/CHERRY_PICK_HEAD`, `.git/REVERT_HEAD`, `.git/BISECT_LOG`; check for `rebase-merge/` and `rebase-apply/` directories | Block all rewrites; show descriptive error message |
