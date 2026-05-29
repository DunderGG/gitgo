# GitGo — Roadmap

## Guiding Principles

- **Incremental delivery:** Each phase produces a usable, testable build.
- **Scope discipline:** Advanced operations (split, squash, reorder) come after the core editing workflow is solid.

---

## Phase 1 — Foundation ✅

> Goal: working skeleton that can open a repo and display its commit log.

- [x] Scaffold Wails v2 project with React + TypeScript + Tailwind
- [x] Set up `go.mod` with `go-git/go-git` dependency
- [x] Implement `git/repo.go`
  - [x] Open a repository from a given path
  - [x] Validate path is a git repository
  - [x] Detect current branch
  - [x] Detect and handle detached HEAD state (return `ErrDetachedHead`)
  - [x] Detect in-progress git operations (`.git/MERGE_HEAD`, `.git/CHERRY_PICK_HEAD`, `.git/REVERT_HEAD`, `.git/BISECT_LOG`, `rebase-merge/`, `rebase-apply/`)
  - [x] Detect remote tracking ref via branch config (`refs/remotes/<remote>/<branch>`)
  - [x] Handle no-remote and no-upstream cases gracefully (mark all commits unpushed, surface notice in `StatusBar`)
  - [x] Compute unpushed commits by walking log from HEAD to upstream tip
- [x] Implement `git/log.go`
  - [x] Walk commit log from HEAD
  - [x] Populate `CommitEntry` list (hash, short hash, first-line message, author name, date, unpushed flag)
  - [x] Limit log depth (configurable, default 100)
- [x] Implement `app/models.go`
  - [x] `RepoInfo` struct
  - [x] `CommitSummary` struct
  - [x] `CommitDetail` struct
  - [x] `EditRequest` struct
  - [x] `OperationResult` struct
- [x] Create `app/app.go` with the `App` struct (all Wails-bound methods return `T` or `(T, error)`)
- [x] Bind `App.SelectDirectory()`, `App.OpenRepository(path)`, and `App.GetCommitLog()` to Wails
  - Note: directory picker is exposed as a bound Go method (`SelectDirectory`) rather than the auto-generated runtime binding, which is regenerated and wiped on each build
- [x] Frontend: `RepoSelector` component with native folder picker
- [x] Frontend: `CommitList` component (read-only, indigo dot = unpushed / grey dot = pushed, legend, column headers)
- [x] Frontend: `StatusBar` component (branch name, no-remote / no-upstream notices, status/error display)
- [x] Frontend: Zustand store wired to backend bindings (`setRepo`, `setStatus`, `setError`, `clearRepo`)
- [x] Write unit tests for `git/` package (14 tests, all passing)
  - [x] Use real temporary on-disk repositories (`t.TempDir()`)
  - [x] `TestOpen_ValidRepo`, `TestOpen_NonRepo`, `TestOpen_DetachedHead`, `TestOpen_MergeInProgress`, `TestOpen_RebaseInProgress`
  - [x] `TestOpen_NoRemote`, `TestOpen_WithUpstream` (exact unpushed count)
  - [x] `TestLog_ReturnsEntries`, `TestLog_RespectsDepthLimit`, `TestLog_ShortHashLength`, `TestLog_IsUnpushedFlag`, `TestLog_DatePopulated`
  - [x] `TestLog_MessageIsFirstLine`, `TestLog_CommitOrder`
- [x] Tighten `.gitignore`: scope `build/` to `build/bin/` only; ignore `frontend/package.json.md5`; add OS noise files

---

## Phase 2 — Core Editing

> Goal: users can edit commit message and date for any unpushed commit.

- [ ] Implement `app/app.go` — `GetCommitDetail(hash)` binding
- [ ] Implement `app/app.go` — `RefreshLog()` binding (re-open current repo and return updated log)
- [ ] Implement `git/rewrite.go`
  - [ ] `AmendCommit` — modify the most recent commit (message, date, author)
  - [ ] `RebaseRewrite` — modify any unpushed commit further back in history
    - [ ] Walk commits from target to HEAD
    - [ ] Apply diff-and-rebuild approach per commit (go-git has no native cherry-pick)
    - [ ] Substitute edited metadata at target position
    - [ ] Reset branch HEAD ref to new tip
    - [ ] Restore original ref on any failure
  - [ ] Detect `git` binary on PATH before auto-stash (go-git has no stash API)
  - [ ] Auto-stash via native `git stash` / `git stash pop` if `git` is available; error clearly if not
- [ ] Bind `App.UpdateCommit(hash, EditRequest)` to Wails
  - [ ] Server-side safety check: reject if commit is not in unpushed set
- [ ] Frontend: `EditPanel` component
  - [ ] Message text area (multi-line)
  - [ ] Date + time picker
  - [ ] Author name and email fields
  - [ ] Fields disabled / hidden for pushed commits
- [ ] Frontend: `ConfirmDialog` component
  - [ ] Display old vs new values side-by-side before confirming
  - [ ] "Apply" and "Cancel" actions
- [ ] Frontend: Refresh `CommitList` after a successful edit
- [ ] Show auto-stash notice in `StatusBar` when applicable
- [ ] Write integration tests for `git/rewrite.go`
  - [ ] Amend HEAD commit message
  - [ ] Amend HEAD commit date
  - [ ] Amend HEAD commit author
  - [ ] Rewrite older unpushed commit (verify full chain is rebuilt)
  - [ ] Reject rewrite of a pushed commit
  - [ ] Verify ref is restored on rewrite failure
  - [ ] Dirty working tree + `git` on PATH → auto-stash succeeds
  - [ ] Dirty working tree + no `git` on PATH → clear error returned

---

## Phase 3 — UX Polish

> Goal: the app feels complete and production-quality for everyday use.

- [ ] Recent repositories list (persisted in `localStorage`)
  - [ ] Store last 10 opened paths
  - [ ] Show in `RepoSelector` with quick-open buttons
  - [ ] Remove entry if path no longer exists
- [ ] Undo last rewrite operation
  - [ ] Record pre-rewrite HEAD ref in memory
  - [ ] Expose `App.UndoLastOperation()` binding
  - [ ] Show "Undo" button in `StatusBar` after each successful edit
- [ ] Branch selector
  - [ ] List local branches
  - [ ] Switch view to selected branch's log
- [ ] Keyboard shortcuts
  - [ ] `Ctrl+Z` — undo last operation
  - [ ] `Enter` on selected commit — open edit panel
  - [ ] `Escape` — close edit panel / dialog
- [ ] Empty state views (no repo open, no unpushed commits, repo with no remote)
- [ ] Loading indicators during git operations
- [ ] Error boundary in frontend with user-friendly messages
- [ ] Application icon and Wails window configuration (title, min size)
- [ ] `CommitList` row selection state (highlight selected commit, drive `EditPanel`)
- [ ] Reload / refresh button in header to re-read the repo from disk

---

## Phase 4 — Advanced Operations

> Goal: cover more complex history editing workflows safely.

- [ ] **Squash commits**
  - [ ] Select multiple contiguous unpushed commits
  - [ ] Combine into one with a merged or custom message
- [ ] **Reorder commits**
  - [ ] Drag-and-drop reordering in `CommitList` for unpushed commits
  - [ ] Detect and surface reorder conflicts
- [ ] **Drop commit**
  - [ ] Remove an unpushed commit from history entirely
  - [ ] Confirmation dialog with strong warning
- [ ] **Split commit** *(stretch goal)*
  - [ ] Reset to pre-commit state, open diff view, let user stage partial changes
- [ ] **Edit commit file tree** *(stretch goal)*
  - [ ] Add / remove files from an unpushed commit

---

## Phase 5 — Distribution

> Goal: ship a binary users can install.

- [ ] Set up GitHub Actions CI pipeline
  - [ ] Run Go tests on push
  - [ ] Run frontend lint + type-check on push
  - [ ] Fail build if `go vet ./...` reports issues
- [ ] Build pipeline for all three platforms
  - [ ] Windows (`.exe` / NSIS installer via Wails)
  - [ ] macOS (`.app` bundle / `.dmg`)
  - [ ] Linux (`.AppImage`)
- [ ] Code-sign macOS binary
- [ ] Version number injected at build time via `ldflags`
- [ ] GitHub Releases with attached platform binaries

---

## Deferred / Out of Scope

These items are explicitly deferred to avoid scope creep in early phases:

- Remote push / pull operations
- SSH or HTTPS credential management
- Viewing or editing file diffs within the app
- Support for multiple open repositories simultaneously
- Submodule awareness
- Git LFS support
