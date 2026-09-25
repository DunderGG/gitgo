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

- [x] Implement `app/app.go` — `GetCommitDetail(hash)` binding
- [x] Implement `app/app.go` — `RefreshLog()` binding (re-open current repo and return updated log)
  - [x] Extract `commitSummariesFromEntries` helper shared by `GetCommitLog` and `RefreshLog`
- [ ] Implement `git/rewrite.go`
  - [x] `AmendCommit` — modify the most recent commit (message, date, author)
  - [x] `RebaseRewrite` — modify any unpushed commit further back in history
    - [x] Walk commits from target to HEAD
    - [x] Apply diff-and-rebuild approach per commit (go-git has no native cherry-pick)
    - [x] Substitute edited metadata at target position
    - [x] Reset branch HEAD ref to new tip
    - [x] Restore original ref on any failure
  - [x] Detect `git` binary on PATH before auto-stash (go-git has no stash API)
  - [x] Auto-stash via native `git stash` / `git stash pop` if `git` is available; error clearly if not
- [x] Bind `App.UpdateCommit(req EditRequest)` to Wails
  - [x] Server-side safety check: reject if commit is not in unpushed set
- [x] Frontend: `EditPanel` component
  - [x] Message text area (multi-line)
  - [x] Date + time picker
  - [x] Author name and email fields
  - [x] Fields disabled / hidden for pushed commits
- [x] Frontend: `ConfirmDialog` component
  - [x] Display old vs new values side-by-side before confirming
  - [x] "Apply" and "Cancel" actions
- [x] Frontend: Refresh `CommitList` after a successful edit
- [x] Show auto-stash notice in `StatusBar` when applicable
- [x] Write integration tests for `git/rewrite.go` (16 tests, all passing)
  - [x] `TestAmendCommit_UpdatesMessage`
  - [x] `TestAmendCommit_UpdatesAuthor`
  - [x] `TestAmendCommit_UpdatesDate`
  - [x] `TestAmendCommit_KeepsTree`
  - [x] `TestAmendCommit_KeepsParents`
  - [x] `TestAmendCommit_RejectsPushedCommit`
  - [x] `TestRebaseRewrite_UpdatesTargetMessage`
  - [x] `TestRebaseRewrite_KeepsNewerCommitContent`
  - [x] `TestRebaseRewrite_PreservesUnchangedParent`
  - [x] `TestRebaseRewrite_TargetIsHead`
  - [x] `TestRebaseRewrite_RejectsPushedCommit`
  - [x] `TestIsDirty_CleanWorktree`
  - [x] `TestIsDirty_DirtyWorktree`
  - [x] `TestAutoStash_StashesDirtyWorktree`
  - [x] `TestAutoStashPop_RestoresChanges`
  - [x] `TestAutoStash_FailsWithBadBinary`

---

## Phase 3 — UX Polish

> Goal: the app feels complete and production-quality for everyday use.

- [x] Recent repositories list (persisted in `localStorage`)
  - [x] Store last 10 opened paths
  - [x] Show in `RepoSelector` with quick-open buttons
  - [x] Remove entry if path no longer exists
- [x] Undo last rewrite operation
  - [x] Record pre-rewrite HEAD ref in memory
  - [x] Expose `App.UndoLastOperation()` binding
  - [x] Show "Undo" button in `StatusBar` after each successful edit
- [x] Branch selector
  - [x] List local branches
  - [x] Switch view to selected branch's log
  - [x] Edit unpushed commits on the selected branch without checking it out (working tree untouched)
- [x] Keyboard shortcuts
  - [x] `Ctrl+Z` — undo last operation
  - [x] `Enter` on selected commit — open edit panel
  - [x] `Escape` — close edit panel / dialog
  - [x] `↑` / `↓` — move selection between commits
- [ ] Empty state views (no repo open, no unpushed commits, repo with no remote)
- [ ] Loading indicators during git operations
- [ ] Error boundary in frontend with user-friendly messages
- [ ] Application icon and Wails window configuration (title, min size)
- [x] `CommitList` row selection state (highlight selected commit, drive `EditPanel`)
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

## Phase 5 — Correctness & Safety Hardening

> Goal: fix the known issues found in the 2026-09-25 review so the app is safe to use on real repositories.
> Until the blocker below is fixed, only use GitGo on branches that are strictly ahead of their upstream (linear history), and create a backup branch first.

- [ ] **Unpushed detection is wrong when history has diverged or contains merges** *(blocker)*
  - `computeUnpushed` in `git/repo.go` walks the log from HEAD and stops when it hits the upstream tip. If the upstream tip is not an ancestor of HEAD (remote moved ahead after a fetch, or local branch is behind), it is never found and **every** commit — including pushed ones — is marked unpushed and editable.
  - After a `git pull` that creates a merge commit, the depth-first walk follows the first (local) parent down past the merge base before reaching the upstream tip, so older pushed commits are also marked unpushed.
  - [ ] Compute unpushed as "reachable from HEAD but not reachable from the upstream tip" (equivalent to `git rev-list HEAD ^@{u}`)
  - [ ] Consider also excluding commits reachable from any `refs/remotes/*` ref, so branches without an upstream don't expose commits already pushed on other branches
  - [ ] Tests: diverged branch (local + remote each have new commits), branch behind upstream, merge commit from `git pull`
- [ ] **Edits silently reset seconds and time zone**
  - `EditPanel` uses a `datetime-local` input (minute precision) and sends `toISOString()` (UTC), so any edit — even message-only — changes the seconds to `:00` and the commit's offset to `+0000`
  - [ ] Preserve seconds (add a seconds field or `step="1"`)
  - [ ] Preserve the original time zone offset (send the date with its original offset, or let the user choose one)
  - [ ] Only send the date when it was actually changed
- [ ] **Committer identity is always overwritten by the author**
  - `AmendCommit` / `RebaseRewrite` set `Committer = Author`, so a message-only edit also replaces the committer name, email, and date
  - [ ] Decide on the intended behaviour (keep original committer, or set committer date to now like `git commit --amend`) and optionally expose committer date in the UI
- [ ] **Rewrites leave no reflog entry**
  - go-git's `SetReference` does not write to the reflog, so `git reflog` cannot be used to recover the pre-rewrite tip
  - [ ] Write a reflog entry for the branch and HEAD on every rewrite (or create a backup ref such as `refs/gitgo/backup/<branch>`)
  - Related: the Phase 3 undo feature
- [ ] **Pushed/unpushed state can go stale**
  - `UnpushedHashes` is computed when the repo is opened; pushing from a terminal while the app is open leaves those commits editable
  - [ ] Re-open / re-validate repo state at the start of `UpdateCommit` before the safety check
- [ ] **Other refs are not updated after a rewrite**
  - Tags or other local branches pointing at a rewritten commit keep pointing at the old commit
  - [ ] Detect such refs and either warn the user or offer to move them

---

## Phase 6 — Distribution

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

## Future Improvements

> Possible improvements we have found but not yet implemented.

### Small polish

- [ ] Copy actions for commit metadata (full hash, short hash, author name, author email)
- [ ] Ahead-of-remote details in the status area (for example: exact number of commits ahead)
- [ ] Persistent app preferences (window size, warning visibility, default UI behavior)
- [ ] Open commit details in an external tool or terminal command
- [ ] Export commit metadata or history summaries as text/JSON for sharing

### Larger additions

- [ ] Show a preview of the Git command that will actually be run
- [ ] Commit search and filtering by message, author, date, or hash
- [ ] Side-by-side commit comparison view
- [ ] Include prerequisites with the app

---

## Deferred / Out of Scope

These items are explicitly deferred to avoid scope creep in early phases:

- Remote push / pull operations
- SSH or HTTPS credential management
- Viewing or editing file diffs within the app
- Support for multiple open repositories simultaneously
- Submodule awareness
- Git LFS support
