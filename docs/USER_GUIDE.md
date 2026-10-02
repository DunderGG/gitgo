# GitGo User Guide

How each feature works and what exactly it changes. For a quick overview, press `F1` or click **?** in the app. Keyboard shortcuts are listed in the [README](../README.md#keyboard-shortcuts).

---

## Opening a repository

Click **Open Repository** and pick the folder of a local Git repository. Recently opened repositories are listed on the start screen.

To switch to another repository, pick it from the repository menu in the header, which lists your recent repositories and **Open another folder…**. Or click **×** next to it to go back to the start screen. Either way, your history is not changed, but the last edit can no longer be undone with **Undo**, and changes you have not applied in the edit panel are lost.

The checked-out branch is shown first. Use the **Branch** menu in the header to view and edit another local branch. That branch is not checked out, and your working tree is left alone.

Made changes outside GitGo, for example a new commit in a terminal? Press `F5` or click **↻** to reload.

The folder button next to **>_** shows the repository folder in Explorer, Finder or your Linux file manager.

## Pushed and unpushed commits

GitGo only edits commits you have not pushed yet. Each row in the commit list starts with a dot:

- **Indigo dot:** unpushed, editable
- **Grey dot, dimmed row:** pushed, read-only. You can still select it to see its details.

A commit counts as pushed when it is on the branch's upstream or on any other remote-tracking branch. Without a remote or an upstream branch, every commit counts as unpushed, and a notice above the list tells you so.

Whether a commit is pushed is checked again right before every edit and undo, so an edit is refused if you pushed in the meantime.

The remote-tracking branches are only as up to date as your last `git fetch` (or `git pull`), and GitGo never fetches by itself. The status bar shows when you last fetched, for example **Fetched 3 days ago**, or **No fetch recorded** for a fresh clone. When the last fetch is older than the limit in [Settings](#settings) (7 days by default), it turns yellow and a notice above the list warns that commits pushed since then, for example from another clone, may still show as unpushed. Run `git fetch` and press `F5` before editing.

## Editing one commit

Click an unpushed commit, or move to it with `↑` / `↓` and press `Enter`. The **Edit Commit** panel shows its details.

### Message

Type the new message. The first line is the subject. If commit message guides are on (see [Settings](#settings)), a ruler marks the subject length limit and a counter turns yellow when the subject is longer.

### Author date

The date is shown in the commit's own time zone, to the second.

- Type a new date and time, or use **−1d**, **−1h**, **+1h**, **+1d** and **Now**. **Now** uses this computer's current time and time zone.
- The time zone menu changes the commit's offset and **keeps the clock time**. For example, `14:00 +02:00` becomes `14:00 +00:00`, which is a different moment.
- The date is only written when you change it. A message-only edit keeps the original date and seconds.
- **Also set committer date** gives the committer date the same value. It starts ticked when the two dates already match.

### Author

Change **Author Name** and **Author Email**, or click **Use my identity** to fill in `user.name` and `user.email` from your Git config. The name can't be empty, and neither field may contain `<`, `>` or a line break, because Git can't store them. **Review Changes** stays disabled until both are valid.

### Committer

Git stores who wrote a commit (the author) and who last applied it (the committer). Under **Committer**, choose one:

- **Keep name and email:** the committer stays as it is. This is the default.
- **Same as author:** the committer becomes the (new) author. It starts this way when the committer was already the author, so fixing the author fixes both.
- **Set name and email:** type a new name and email.

The committer date only changes when **Also set committer date** is ticked.

Click **Review Changes** to continue, or **Reset** to put the fields back.

## Editing several commits

Select several unpushed commits:

- `Ctrl`+click adds or removes one commit.
- `Shift`+click selects the range from the last selected commit.
- `Shift`+`↑` / `↓` extends the selection.
- **Select all unpushed**, or `Ctrl+A`, selects every unpushed commit.

The **Edit Several Commits** panel replaces the edit panel. All of its changes are applied as one rewrite, which one undo reverts. Messages can't be changed for several commits at once.

### Dates

The **Dates** switch chooses how the author dates change: **Shift** moves every commit by the same amount, and **Spread** fits the commits between a first and last date. You can switch back and forth: each mode keeps what you entered, and only the selected mode is applied.

**Also move committer dates** (on by default) moves each commit's committer date by as much as its author date moves, so the gap between the two stays the same. When it is off, the committer dates are kept.

If the new dates would date a commit earlier than the commit below it in the list, the panel warns you. This can happen at the edges of the selection, next to commits you didn't select. The edit is still allowed.

#### Shift

Use **−1d**, **−1h**, **+1h** and **+1d** to build up a shift. Every selected commit moves by the same amount and keeps its own time zone. **Reset** goes back to no shift.

#### Spread over a range

Use this to backdate a series of commits, or to space them out, without every commit sharing the same shift.

- **First** is the new date of the oldest selected commit and **Last** the new date of the newest. Both are applied exactly. They start at those commits' current dates, so nothing changes until you edit them.
- Commits are ordered by their place in the history, not by their current dates, and stay in that order.
- **Keep relative spacing** (the default) keeps the pattern of the current dates and stretches or squeezes it to fit the range. Commits made minutes apart stay close together, and a long break stays long. For example, three commits made at 09:00, 09:10 and 10:00, spread from 13:00 to 15:00, become 13:00, 13:20 and 15:00.
- **Even** puts the same time between every commit. Three commits from 13:00 to 15:00 become 13:00, 14:00 and 15:00.
- **Random** places the commits between the first and last date at random, like real work: some end up close together, others far apart. No two commits are ever closer than the **Minimum gap** (10 minutes unless you change it), the order is kept, and the first and last dates stay exact. The random dates are drawn once and shown in the list, and those exact dates are applied. **Re-roll** draws new ones.
- If the current dates are all the same, or out of order (for example after an earlier edit), their pattern can't be kept. **Keep relative spacing** then spaces the commits evenly instead, and the panel tells you so. **Random** is often the better choice in that case.
- Dates are whole seconds, as Git stores them, so a gap that doesn't divide evenly is rounded to the nearest second.
- The last date must be after the first. With **Even**, the range needs at least one second per gap. With **Random**, it needs room for the minimum gaps: 5 commits at least 10 minutes apart need at least 40 minutes between the first and last date. If the range is exactly that long, the commits end up evenly spaced.
- **Time zones:** First and Last are moments in time, entered with their own UTC offset. Each commit keeps its own time zone, so a commit in a different zone gets the same moment shown at its own clock time. For example, a First of `09:00 +02:00` becomes `07:00 +00:00` on a commit made in UTC. The panel notes when this applies.

##### Only office hours

Without it, every hour of the range counts, so a spread over several days can put commits at night or at the weekend. Tick **Only office hours** to keep every commit within your working hours. The label shows the hours in use, for example `(Mon–Fri, 09:00–17:00)`, and **Change** opens the settings where you set them (see [Settings](#settings)).

- Time outside office hours is skipped, as if the range were only the office hours joined together. This applies to all three spacings: **Even** puts the same amount of *office* time between commits, **Random** never places a commit outside office hours and counts the minimum gap in office time, and **Keep relative spacing** fits the current pattern into office time.
- For example, with Mon–Fri 09:00–17:00, five commits spread evenly from Friday 16:00 to Monday 10:00 have two hours of office time, so they land at Friday 16:00, 16:30 and 17:00, then Monday 09:30 and 10:00. The night and the weekend are skipped.
- A gap can therefore run across a night: the list shows the real time between commits, for example `(+16h 30m)` from 17:00 to 09:30 the next morning.
- Commits can land exactly at the opening or closing time.
- **First** and **Last** must be within office hours, on a working day. They are applied exactly, so GitGo won't move them; the panel tells you if one is outside.
- The office hours are read in the time zone of **First**. A commit in another time zone still gets the same moment, shown at its own clock time.
- A range that is too short in office time is refused, just as for a range that is too short overall, for example "5 commits at least 10m apart need at least 40m of office hours between the first and last date".

The list at the bottom of the panel shows each commit's new date, with the time since the next older selected commit in brackets, for example `(+1d 4h 53m)`. The confirmation dialog shows the same.

### Author and committer

Tick **Set Author** and type a name and email, or click **Use my identity**. Every selected commit gets that author. Under **Committer**, choose **Keep name and email**, **Same as each commit's author** (its new author, if you set one), or **Set name and email** to give all of them the same committer.

The list at the bottom of the panel shows each commit's new date, author and committer before you review.

## Reviewing and applying

**Review Changes** opens a confirmation dialog with the current and new values side by side, changes highlighted. Click **Apply** to rewrite the history, or **Cancel** to go back.

The dialog also warns you when:

- **Signed commits** would lose their GPG or SSH signature. A signature only matches the exact commit it was made for, and GitGo can't re-sign commits yet.
- **Other branches** point at a rewritten commit. Tick the box to move them along with the edit.
- **Tags** point at a rewritten commit. Tags are never moved and keep pointing at the old commit.

**Back up first**, at the bottom of the dialog, saves the branch, and every other branch the edit moves, as an automatic backup before applying, so you can [restore](#backups) them later even after undo is no longer possible. It starts ticked; change that in the settings.

### What a rewrite changes

- The edited commit and every commit above it get new hashes, because a commit's hash covers its parent. Their content (the files) doesn't change.
- Only the branch moves. Uncommitted changes, staged or not, and the stash are never touched.
- Extra commit headers such as `encoding` and `mergetag` are kept. Signatures are dropped, as described above.

## Undo and recovery

Click **Undo** in the status bar or press `Ctrl+Z` to put the branch back to how it was before the last rewrite. Only the most recent rewrite can be undone, and switching branches clears it. Undo fails, and changes nothing, if the branch changed on disk since the rewrite.

Every rewrite and undo is also written to the reflog, with messages starting with `gitgo:`. From the command line you can always go back one step with:

```bash
git reset --hard <branch>@{1}
```

### Backups

Undo only covers the last rewrite and is gone when you switch branch or close GitGo. For more, click **Back up** in the header, optionally type a name such as "before date spread", and press **Enter**: it saves the branch as it is now, instantly, whatever the repository size. Pressing **Enter** without typing makes an unnamed backup; **Escape** cancels. Backups stay in the repository until you delete them, survive `git gc`, and are never pushed.

The clock button next to **Back up** opens the list of backups of the branch you are viewing; tick **Show all branches** to see the others too. Each backup shows when it was made, whether you made it (**Manual**) or GitGo did (**Automatic**), and the commit it saved for each branch, marked **same as now** when the branch still points there.

- **Name** (or **Rename**) on a backup sets or changes its name, also for automatic backups; clear the field to remove it. Names can be up to 100 characters on one line, and are kept when a backup is exported.
- **Restore…** shows, per branch, the commits that would be removed from it and the ones that would come back. Click **Restore** to move the branches back. The current state is saved as an automatic backup first, and `Ctrl+Z` undoes the restore.
- **Delete** removes the backup after you confirm.
- **Export…** saves the backup to a git bundle file outside the repository, with each branch's whole history, so it survives the repository being deleted or cloned again. Click **Save…** to pick the file. It needs git installed, like the [Run menu](#run-menu).

  Tick **Only commits not on a remote** for a much smaller file that leaves out everything already pushed. Such a file can only be fetched into a clone that already has the pushed history, so it does not protect you if the repository is lost; and when every commit in the backup is on a remote, there is nothing to export. The checkbox is disabled in a repository without a remote.

  To get the backup back, in the original repository or a fresh clone, run the command below; it then appears in the Backups list again:

  ```bash
  git fetch <file>.bundle 'refs/gitgo/backups/*:refs/gitgo/backups/*'
  ```

A restore changes nothing and explains why when:

- it would remove commits that have been **pushed**;
- the branch is **checked out** and commits made since the backup changed files: moving the branch back would leave those changes behind as uncommitted changes. Use the terminal (`git reset --hard`) if you really want to discard them;
- a branch **changed** while you were reviewing; review again.

A branch deleted since the backup is not recreated. Backups are refs under `refs/gitgo/backups/`, so they can also be used from the command line:

```bash
git for-each-ref refs/gitgo/backups          # list backups
git reset --hard refs/gitgo/backups/<id>/main  # restore one by hand
```

## Run menu

The **Run** menu in the header runs a few ready-made git commands, so common tasks need no terminal. None of them change the repository.

- **History of all commits in the list:** the commits the list shows, newest first, as **One line per commit** (short hash, date, author and subject), **Full metadata** (author and committer with their dates, and the whole message), or **Authors and dates (CSV)** (hash, author, author email, author date, committer, committer email, committer date and subject, for a spreadsheet).
- **History of the selected commits:** the same, for the commits you selected, in list order.
- **Export → selected commits as patches…:** pick a folder, and each selected commit is written there as a patch file (`git format-patch`), numbered from the oldest, for example `0001-fix-typo.patch`. Merge commits get no patch.

The output opens in a window showing the command that ran. Click **Copy** to put the output on the clipboard, or **Save…** to save it to a file. Very long output is cut off at 10 MB.

The menu needs git installed, unlike the rest of GitGo. Without it, the menu is disabled, and hovering over it says why. Install git, then reopen the repository.

## Settings

Open the settings with the **⚙** button in the header or `Ctrl+,`. Changes apply at once and are saved for next time.

- **Theme:** System (follows your operating system's setting), Light or Dark. The **◐** button in the header switches between them too.
- **Commit message guides:** the subject line length (default 50) draws a ruler in the message field and warns about longer subjects. The body line length (default 72) warns about longer body lines. 0 turns either one off.
- **Office hours:** your working hours, From and to, and the working days (default Mon–Fri, 09:00–17:00). They are used by **Only office hours** when [spreading dates](#only-office-hours). The start must be before the end, on the same day, so hours that run past midnight aren't supported, and at least one day must be picked.
- **Backups:** whether **Back up first** starts ticked when you review an edit (default on), and how many automatic backups to keep per branch (default 20). Older automatic backups beyond that number are deleted when a new one is made; backups you make with **Back up** are kept until you delete them. See [Backups](#backups).
- **Remote:** how many days after your last fetch GitGo warns that the [pushed and unpushed split](#pushed-and-unpushed-commits) may be out of date (default 7). 0 turns the warning off; the status bar still shows when you last fetched.
- **Terminal command:** what the **>_** button in the header runs. `{dir}` stands for the repository folder. Leave it empty to use the default: Windows Terminal (or cmd) on Windows, Terminal on macOS, and `$TERMINAL` or the first common terminal found on Linux. Running git in that terminal needs git installed; GitGo itself doesn't.
