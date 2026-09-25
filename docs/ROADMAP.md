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
  - [x] ~~Detect `git` binary on PATH before auto-stash (go-git has no stash API)~~ *(auto-stash removed in Phase 4)*
  - [x] ~~Auto-stash via native `git stash` / `git stash pop` if `git` is available; error clearly if not~~ *(removed in Phase 4: rewrites never touch the working tree)*
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
- [x] ~~Show auto-stash notice in `StatusBar` when applicable~~ *(removed with auto-stash in Phase 4)*
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
  - [x] ~~`TestIsDirty_CleanWorktree`~~ *(removed with auto-stash)*
  - [x] ~~`TestIsDirty_DirtyWorktree`~~ *(removed with auto-stash)*
  - [x] ~~`TestAutoStash_StashesDirtyWorktree`~~ *(removed with auto-stash)*
  - [x] ~~`TestAutoStashPop_RestoresChanges`~~ *(removed with auto-stash)*
  - [x] ~~`TestAutoStash_FailsWithBadBinary`~~ *(removed with auto-stash)*

---

## Phase 3 — UX Polish ✅

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
- [x] Empty state views (no repo open, no unpushed commits, repo with no remote)
  - [x] Also: branch with no upstream, and a clear `ErrNoCommits` for a repository with no commits yet
- [x] Loading indicators during git operations
  - [x] Shared `activity` state: status-bar spinner, and only one git operation at a time
- [x] Error boundary in frontend with user-friendly messages
  - [x] Friendly mapping for raw backend errors, with the raw text kept as a tooltip
  - [x] Uncaught promise rejections shown in `StatusBar`
- [x] Application icon and Wails window configuration (title, min size)
  - [x] Square `appicon.png`; icon embedded for Linux and the macOS About panel
  - [x] Dark native title bar on Windows and macOS
  - [x] Window title shows the open repository and branch
- [x] `CommitList` row selection state (highlight selected commit, drive `EditPanel`)
- [x] Reload / refresh button in header to re-read the repo from disk
  - [x] `F5` / `Ctrl+R` reload the repository instead of the webview

---

## Phase 4 — Correctness & Safety Hardening ✅

> Goal: fix the known issues found in the 2026-09-25 review so the app is safe to use on real repositories.

- [x] **Unpushed detection is wrong when history has diverged or contains merges** *(blocker)*
  - `computeUnpushed` in `git/repo.go` walks the log from HEAD and stops when it hits the upstream tip. If the upstream tip is not an ancestor of HEAD (remote moved ahead after a fetch, or local branch is behind), it is never found and **every** commit — including pushed ones — is marked unpushed and editable.
  - After a `git pull` that creates a merge commit, the depth-first walk follows the first (local) parent down past the merge base before reaching the upstream tip, so older pushed commits are also marked unpushed.
  - [x] Compute unpushed as "reachable from HEAD but not reachable from the upstream tip" (equivalent to `git rev-list HEAD ^@{u}`)
  - [x] Also exclude commits reachable from any `refs/remotes/*` ref, so branches without an upstream don't expose commits already pushed on other branches
  - [x] Tests: diverged branch (local + remote each have new commits), branch behind upstream, merge commit from `git pull`, no-upstream branch with pushed history, up-to-date branch
     - Note: the remote history is walked completely (no commit-date cut-off) so clock skew can never mark a pushed commit editable; this may be slow on very large repositories
- [x] **Edits silently reset seconds and time zone**
  - `EditPanel` uses a `datetime-local` input (minute precision) and sends `toISOString()` (UTC), so any edit — even message-only — changes the seconds to `:00` and the commit's offset to `+0000`
  - [x] Preserve seconds (`step="1"` on the date input)
  - [x] Preserve the original time zone offset: the date is shown and edited in the commit's own offset, with a selector to change it
  - [x] Only send the date when it was actually changed (an empty date keeps the original author date in `AmendCommit` / `RebaseRewrite`)
- [x] **Committer identity is always overwritten by the author**
  - `AmendCommit` / `RebaseRewrite` set `Committer = Author`, so a message-only edit also replaces the committer name, email, and date
  - [x] Decided: keep the original committer name, email, and date; an "Also set committer date" checkbox in `EditPanel` sets the committer date to the author date (checked by default when the two dates already match)
  - [x] Show the committer (read-only) in `EditPanel` and the committer date in `ConfirmDialog`
- [x] **Rewrites leave no reflog entry**
  - go-git's `SetReference` does not write to the reflog, so `git reflog` cannot be used to recover the pre-rewrite tip
  - [x] Write a reflog entry for the branch (and HEAD when it points at the branch) on every rewrite, as `gitgo: edit commit <hash>`; recover with `git reset --hard <branch>@{1}`
  - [x] Undo is recorded too (`gitgo: undo edit`), and every branch move is now a compare-and-swap that fails if the branch changed on disk
  - [x] Honour `core.logAllRefUpdates=false` (only append to existing reflogs)
  - Related: the Phase 3 undo feature
- [x] **Pushed/unpushed state can go stale**
  - `UnpushedHashes` is computed when the repo is opened; pushing from a terminal while the app is open leaves those commits editable
  - [x] Re-open / re-validate repo state at the start of `UpdateCommit` before the safety check; the edit panel then shows the commit as pushed
  - [x] Undo is refused (`ErrUndoPushed`) when any commit it would discard has been pushed since the edit
- [x] **Other refs are not updated after a rewrite**
  - Tags or other local branches pointing at a rewritten commit keep pointing at the old commit
  - [x] Detect such refs (`FindAffectedRefs`) and list them in `ConfirmDialog`
  - [x] Decided: move local branches that point at a rewritten commit (checkbox, on by default, like `git rebase --update-refs`); undo moves them back
  - [x] Tags are never moved, only warned about; branches with their own commits on top of a rewritten commit are warned about (they keep the old history)

Found in the follow-up review (2026-09-25):

- [x] **Auto-stash could pop an unrelated stash and lost staging** *(blocker)*
  - go-git's `Worktree.Status()` reports a clean Windows worktree as dirty when a tracked file has the executable bit (e.g. `gradlew`, `*.sh` committed from Linux/macOS), because it ignores `core.fileMode=false`. `git stash` then saved nothing, and the following `git stash pop` applied and dropped the user's older, unrelated stash.
  - Even when a stash was created, `git stash pop` without `--index` turned staged changes into unstaged ones.
  - [x] Decided: remove auto-stash entirely. Edits only change commit metadata, never file trees, so the index and working tree stay consistent when the branch moves; uncommitted work (staged or not) and the stash are never touched. Removed `IsDirty`, `FindGitBinary`, `AutoStash`, `AutoStashPop` and `ErrNativeGitNotFound`; the native `git` binary is no longer a runtime dependency.
  - [x] Tests: `TestUpdateCommit_KeepsStashWhenWorktreeIsClean`, `TestUpdateCommit_KeepsStagedAndUnstagedChanges` (both fail against the old auto-stash code)
- [x] **Rebuilt commits dropped the `encoding`, `mergetag` and other extra headers**
  - [x] `rebuildCommit` keeps them on the edited commit and on every commit rebuilt above it (`TestRebaseRewrite_KeepsEncodingHeader`); signatures (`gpgsig*`) are still dropped, see Future Improvements
- [x] **Author name and email were not validated**
  - An empty name, or `<`, `>` or a line break in the name or email, produced a malformed commit header that `git fsck` and many servers reject on push
  - [x] `AmendCommit` / `RebaseRewrite` return `ErrInvalidIdentity` (`TestAmendCommit_RejectsInvalidIdentity`, `TestUpdateCommit_RejectsInvalidAuthor`); `EditPanel` shows the same rules inline and disables "Review Changes"

---

## Phase 5 — Advanced Operations

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
- [ ] **Reword several messages in one view**: a list of the selected commits' messages edited side by side and applied in one rewrite, like the reword step of `git rebase -i` (as in lazygit and Sublime Merge)
- [ ] **Undo last commit**: remove the newest unpushed commit and keep its changes staged, like `git reset --soft HEAD~1` (GitHub Desktop's "Undo" button)
- [ ] **Apply fixup commits**: fold `fixup!` / `squash!` commits into their targets, like `git rebase -i --autosquash`
- [ ] **Move commits to another branch**: take unpushed commits off this branch and put them on a new or existing one, for commits made on the wrong branch (as in Fork and GitKraken)

---

## Phase 6 — Distribution

> Goal: ship a binary users can install.

- [x] Set up GitHub Actions CI pipeline (`.github/workflows/ci.yml`)
  - [x] Run Go tests on push and pull requests (Ubuntu + Windows)
  - [x] Run frontend type-check and production build on push and pull requests
  - [x] Run frontend lint (ESLint, zero warnings allowed) on push and pull requests
  - [x] Fail build if `go vet ./...` reports issues
- [ ] Build pipeline for all three platforms
  - [ ] Windows (`.exe` / NSIS installer via Wails)
  - [ ] macOS (`.app` bundle / `.dmg`)
  - [ ] Linux (`.AppImage`)
- [ ] Code-sign macOS binary
- [ ] Version number injected at build time via `ldflags`
- [ ] GitHub Releases with attached platform binaries
- [ ] Include prerequisites with the app

---

## Future Improvements

> Possible improvements we have found but not yet implemented, grouped by the part of the app they touch.

### Commit editing

Editing commit metadata in `EditPanel` and `BulkDatePanel`.

- [ ] **Use my identity** button by the author fields in `EditPanel`, filling in `user.name` / `user.email` from the Git config (the lookup already exists for reflog entries in `git/reflog.go`). The most common reason to change an author is a commit made with the wrong identity
- [ ] **Set author on several commits** in `BulkDatePanel`, next to the date shift, with the same Use my identity button, so a batch of wrongly attributed commits is fixed in one rewrite
- [ ] **Spread dates evenly** in `BulkDatePanel`: set a first and last date and space the selected commits between them, for backdating a series without making every commit share the same shift
- [ ] **Keep dates within a time window**: move the selected commits' times into a daily window (for example 09:00–17:00), keeping their order and days (like `git-redate` and similar scripts)
- [ ] **Change time zone, keeping the moment**: convert the selected commits' dates to another offset (for example "my time zone") without changing the actual point in time, unlike the offset menu in `EditPanel`, which keeps the wall-clock time
- [ ] **Edit the committer**: an option to set the committer name and email too, or to reset them to the author, since a wrong identity usually affects both. Today the committer is always kept
- [ ] **Apply `.mailmap`**: rewrite the author (and committer) of the selected commits from the repository's `.mailmap`, like `git filter-repo --use-mailmap`
- [ ] **Find and replace in messages** across the selected commits, with a preview per commit (like `git filter-repo --replace-message`), for example to fix a misspelled ticket number
- [ ] **Trailers**: add or remove `Co-authored-by:` and `Signed-off-by:` lines on one or several commits, with co-authors picked from the repository's authors (GitHub Desktop has a co-author picker)
- [ ] **Message guides**: a ruler at 50 characters for the subject and 72 for the body, and a warning when the second line is not blank (as in Sublime Merge and Tower)
- [x] Add tiny date/time buttons under the date field to add +1 hour, +1 day, current time, etc.
  - [x] `EditPanel` has −1d, −1h, +1h, +1d (shift the wall-clock time, keeping the offset) and Now (current time and this computer's offset)
- [x] Shift the dates of several selected commits at once (multi-select in `CommitList`, ±1h/±1d in a bulk panel)
  - [x] `RewriteCommits`: edit any set of unpushed commits in one chain rebuild, so one undo and one reflog entry cover the batch (`TestRewriteCommits_*`); `AmendCommit` / `RebaseRewrite` now wrap it
  - [x] `ShiftCommitDates` bound method (relative shift per commit, keeping each commit's offset; optionally shift committer dates) and `GetAffectedRefs` for several commits (`TestShiftDates_*`, `TestShiftCommitDates_*`)
  - [x] Multi-select in `CommitList` (Ctrl/Shift-click, Shift+↑/↓; unpushed commits only) and `BulkDatePanel` with a per-commit preview in `ConfirmDialog` (now a generic frame; `CommitComparison` holds the single-commit rows)

### Commit list and selection

Finding, selecting and inspecting commits in `CommitList`.

- [ ] **Select all unpushed** (button above `CommitList` and `Ctrl+A`), as the starting point for a bulk shift or author fix
- [ ] Commit search and filtering by message, author, date, or hash
- [ ] Side-by-side commit comparison view
- [ ] Copy actions for commit metadata (full hash, short hash, author name, author email)
- [ ] **Load more commits**: the list stops at 100 commits (`defaultLogDepth` in `git/log.go`); load the next page when scrolling to the end, or show a "Load more" row
- [ ] **Branch and tag labels** on the rows they point to, as every Git GUI does, so it is clear which other refs an edit will affect before opening `ConfirmDialog`
- [ ] **Commit graph**: draw the branch and merge lines next to the list (as in Fork, GitKraken and `git log --graph`), making merge commits and where the pushed part starts easier to see
- [ ] **Changed files list** (names and +/− line counts, read-only) in the commit details. This is much lighter than the file diffs that are out of scope, and helps confirm the right commit is selected
- [ ] **Relative dates** ("3 hours ago") as an option, with the full date and offset in a tooltip
- [ ] **Flag odd commits** in the list: author date later than the commit above it, committer different from the author, or a date in the future
- [ ] **Right-click menu** on commit rows with Edit, Copy hash and, later, Squash / Drop / Reword (every desktop Git client has one)

### Safety and recovery

Protecting history and getting back to an earlier state.

- [ ] **Backups**: save the branch's history before an edit and restore it later, persistently and with multiple steps, unlike the one in-memory `Ctrl+Z` step. Design: [BACKUPS.md](BACKUPS.md)
  - [ ] `git/backup.go`: create, list, delete and restore backup refs under `refs/gitgo/backups/` (never pushed, kept by `git gc`)
  - [ ] Bound methods `CreateBackup`, `ListBackups`, `RestoreBackup`, `DeleteBackup`
  - [ ] **Back up** button in the header and a **Backups** list with Restore and Delete (this replaces the reflog-based history panel idea)
  - [ ] **Back up before applying** checkbox in `ConfirmDialog` (on by default), keeping the last ~20 automatic backups per branch
  - [ ] Back up every branch an edit moves, so a restore brings them all back
  - [ ] Optional: export a backup to a `git bundle` file
- [ ] **Redo** after an undo (`Ctrl+Shift+Z` / `Ctrl+Y` and a button in the status bar), by keeping the undone tip the way `lastRewrite` keeps the pre-rewrite one
- [ ] Handle signed commits: an edit silently drops the GPG/SSH signature of the edited commit and of every commit rebuilt above it (a copied signature would no longer verify)
  - [x] Warn in `ConfirmDialog` when any commit that will be rebuilt is signed (`FindSignedCommits` / `GetSignedCommits`, which also detect `gpgsig-sha256`, which go-git does not parse; `TestFindSignedCommits_*`)
  - [ ] Optionally re-sign rebuilt commits when `commit.gpgSign` is set (like `git rebase` does), via the native `git` / `gpg` binaries
- [ ] Show a preview of the Git command that will actually be run
- [ ] **Stale remote warning**: pushed/unpushed detection uses the local remote-tracking branches, which are only as fresh as the last `git fetch`. Show when the last fetch happened (from the time `FETCH_HEAD` was last written) and warn when it is old, without GitGo fetching itself
- [ ] **Protected branches** setting: never allow edits on chosen branches (for example `main`), even when unpushed

### Repository and branch status

Opening repositories and showing their state in the header and `StatusBar`.

- [ ] **Close / switch repository** button in the header. Once a repository is open there is no way back to the start screen and its recent list; `clearRepo` exists in the store but only `ErrorBoundary` calls it
- [ ] Ahead-of-remote details in the status area (for example: exact number of commits ahead)
- [ ] **Open a repository from the command line** (`gitgo <path>`), so it can be started from a terminal or used as an external tool in an editor
- [ ] **Drag and drop** a folder onto the window to open it (as in GitHub Desktop)
- [ ] **Watch the repository** for changes made outside GitGo and offer to reload, instead of relying on `F5`
- [ ] **Worktrees**: list the repository's linked worktrees (`git worktree`) and open them, noting which branch each has checked out

### External tools and export

Handing the repository or its data to other programs.

- [ ] **Open folder** button next to the terminal button, showing the repository in Explorer / Finder / the Linux file manager
- [ ] Open commit details in an external tool or terminal command
- [ ] Export commit metadata or history summaries as text/JSON for sharing
- [ ] **Open in editor** button, opening the repository in the user's editor (VS Code, or the one set in `core.editor` / an app preference)
- [ ] **View on GitHub / GitLab / Bitbucket** for pushed commits, building the commit URL from the remote URL
- [ ] **Export as patches**: save the selected commits as `.patch` files, like `git format-patch`
- [x] Header button that opens a terminal in the repository folder (`OpenTerminal`; Windows Terminal or cmd, Terminal.app, `$TERMINAL` or a common Linux emulator)

### App settings and help

- [ ] Persistent app preferences (window size, warning visibility, default UI behavior)
- [ ] **Command palette** (`Ctrl+Shift+P`) listing every action with its shortcut, as in Sublime Merge and GitKraken
- [ ] **Light theme** and following the system theme; the app is dark only today (`windows.Dark` in `main.go`, dark Tailwind classes throughout)
- [ ] **Zoom** with `Ctrl +` / `Ctrl −` / `Ctrl 0`, for small or high-DPI screens
- [ ] **First-run tour** highlighting the commit list, the edit panel and the Undo button, reusing the `HelpDialog` content
- [ ] **Update check** against GitHub Releases, once Phase 6 publishes binaries
- [ ] **Translations**: move UI strings into one place so the app can be localised
- [x] In-app help (`HelpDialog`, `?` button in the header or `F1`): walkthrough of single and bulk edits, review and undo, keyboard shortcuts and safety notes

---

## Deferred / Out of Scope

These items are explicitly deferred to avoid scope creep in early phases:

- Remote push / pull operations
- SSH or HTTPS credential management
- Viewing or editing file diffs within the app
- Support for multiple open repositories simultaneously
- Submodule awareness
- Git LFS support
