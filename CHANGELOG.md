# Changelog

What changed in each GitGo release. When a version tag is pushed, the release workflow uses the section whose heading matches the tag (`## [1.2.0]` for `v1.2.0`) as the notes of the [GitHub release](https://github.com/DunderGG/gitgo/releases), followed by a guide to the downloads.

## [0.1.0] - 2026-10-31

The first release of GitGo: a desktop app for fixing the message, date and author of commits you haven't pushed yet, without memorising git commands.

### Highlights

- **Edit a commit:** its message, author, committer and author date, and add co-authors and sign-offs.
- **Edit several commits at once:** set the author or committer on all of them, add or remove co-authors and sign-offs, or spread their dates over a time range, optionally within your office hours only.
- **Find commits:** search by message, author, hash or date, with `author:`, `since:` and other qualifiers, then select the matches and edit them together.
- **Review before applying:** every change is shown old-vs-new before anything is written.
- **Undo and backups:** `Ctrl+Z` undoes the last rewrite, and backups save a branch's history so you can restore it later, or export it to a file.
- **Push only some commits:** get a `git push` command for the older commits and keep working on the newer ones.
- **Any local branch:** view and edit other branches without checking them out.
- **Run menu:** the history as text or CSV, or the selected commits as patch files.
- Light and dark themes, zoom, keyboard shortcuts, and in-app help behind `F1`.

### Safety

- Only unpushed commits can be edited. Pushed history is read-only, and GitGo checks again right before every edit.
- Uncommitted changes, staged or not, and the stash are never touched.
- Every rewrite is written to the reflog, so `git reset --hard <branch>@{1}` always takes you back one step.
- No `git` install is needed. Only the Run menu and exporting backups use it.

### Known limitations

- Editing a signed commit drops its GPG or SSH signature, and those of the commits above it. GitGo warns you before applying.
- GitGo is not code-signed yet, so Windows and macOS ask you to confirm the first launch.
