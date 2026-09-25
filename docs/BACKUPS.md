# Backups: design proposal

> **Status:** proposed, not implemented. Tracked in [ROADMAP.md](ROADMAP.md) under *Larger additions*.

Let the user save a backup of a branch's history before editing it (for example before shifting dates), and restore that backup later if something goes wrong.

---

## What exists today

- **Undo (`Ctrl+Z`)** reverts only the last rewrite. The record (`lastRewrite` in `app/models.go`) lives in memory, so it is gone once the app closes, the branch is switched or a second edit is applied.
- **Reflog.** Every rewrite and undo writes a `gitgo:` reflog entry (`git/reflog.go`), so `git reset --hard <branch>@{1}` recovers the previous state. That only works from the command line, and `git gc` expires reflog entries: unreachable ones after 30 days and reachable ones after 90, by default.

Neither gives the user a safety net they can rely on from inside GitGo.

---

## How it would work

A GitGo rewrite never deletes the old commits: it creates new ones and moves the branch to them. So a backup does not need to copy anything.

### Back up

Create a ref pointing at the branch's current tip, for example:

```
refs/gitgo/backups/main/2026-09-25T14-03-00
```

This is instant, whatever the repository size. As long as the ref exists, `git gc` keeps every commit it points to, with no expiry.

### Restore

Move the branch back to the backup's commit. This is the same operation undo already performs (`ResetBranch` in `git/undo.go`):

- **Compare-and-swap:** nothing changes if the branch moved since it was read.
- **Pushed check:** refused if it would discard commits that have been pushed.
- **Reflog:** recorded as a `gitgo:` entry like every other GitGo branch update.

### Where backups live

Backups go in their own `refs/gitgo/backups/<branch>/<timestamp>` namespace, not in ordinary branches (such as `gitgo-backup/main-…`), so they:

- do not clutter `git branch` or the `BranchSelector`
- are not pushed by `git push`, even with `--all` (which pushes only `refs/heads/`)
- can still be used from the command line:

```bash
git for-each-ref refs/gitgo/backups          # list backups
git reset --hard refs/gitgo/backups/main/…   # restore one by hand
git update-ref -d refs/gitgo/backups/main/…  # delete one
```

### UI

- **Back up** button in the header, for the current branch.
- **Back up before applying** checkbox in `ConfirmDialog`, on by default, so every edit gets an automatic backup.
- **Backups** list showing each backup's date, branch, commit count and whether it was manual or automatic, with **Restore** and **Delete** buttons. Restore goes through a confirmation dialog like every other rewrite.
- **Retention:** keep manual backups until deleted; keep only the last ~20 automatic backups per branch. The limit could become a preference later.

---

## Limitations to design around

### 1. New commits made after the backup

Restoring discards commits made since the backup, for example ones made in a terminal.

- **Branch not checked out:** harmless; only the ref moves.
- **Checked-out branch:** those commits' file changes would show up as uncommitted changes in the working tree, which is confusing. Undo avoids this because a rewrite never changes file trees, so moving the ref leaves the index and working tree consistent.

**Rule:** on the checked-out branch, allow a restore only when the current tip's tree matches the backup tip's tree, meaning only metadata changed since the backup. Otherwise, explain why and point the user to the terminal button. In every case the confirmation dialog lists what will be discarded, for example "3 commits made since this backup will be removed".

### 2. Backups are local

Backup refs live inside the repository's `.git` folder. Deleting or re-cloning the repository deletes them, and `git clone` does not copy them. Exporting a backup to a file (a `git bundle`) would cover this; it is left for later, and it still needs checking whether go-git can create bundles.

### 3. Other branches and tags

An edit can move other local branches along with it (the "Also move … to the edited commits" option in `ConfirmDialog`). A backup of one branch would not bring those back. A backup should therefore record every branch the edit touches, for example as one ref per branch under a shared backup name:

```
refs/gitgo/backups/2026-09-25T14-03-00/main
refs/gitgo/backups/2026-09-25T14-03-00/feature-x
```

Restore then moves them together, with the same compare-and-swap and pushed checks for each. This layout replaces the per-branch one shown above. Tags are never moved by GitGo, so they need no backup.

---

## Relation to other roadmap items

- **GitGo history panel:** backups replace the reflog-based history panel idea. The panel lists backups instead of reflog entries, and those do not expire.
- **Undo** stays as the quick one-step shortcut; backups are the persistent, multi-step safety net.

## Suggested implementation order

1. `git/backup.go`: create, list, delete and restore backup refs, with tests for gc safety, the compare-and-swap, the pushed check and the tree-match rule.
2. Bound methods in `app/`: `CreateBackup`, `ListBackups`, `RestoreBackup`, `DeleteBackup`.
3. Manual **Back up** button and a **Backups** list with Restore and Delete.
4. Automatic backup before each apply, with retention.
5. Multi-branch backups (limitation 3).
6. Optional: export to a bundle file.
