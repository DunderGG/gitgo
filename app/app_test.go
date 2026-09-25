package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "gitgo/git"
)

// runGit runs a git command in dir with a fixed identity and returns its
// trimmed output, failing the test on error.
func runGit(test *testing.T, dir string, args ...string) string {
	test.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test Author",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		test.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitFile writes a file named after message and commits it.
func commitFile(test *testing.T, dir, message string) {
	test.Helper()
	if err := os.WriteFile(filepath.Join(dir, message+".txt"), []byte(message), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}
	runGit(test, dir, "add", ".")
	runGit(test, dir, "commit", "-m", message)
}

// setupRepoWithUnpushedCommit creates a repo whose "base" commit is pushed to
// an origin remote and whose "local" commit on top is not. It returns the
// repo directory and an App with the repository open.
func setupRepoWithUnpushedCommit(test *testing.T) (string, *App) {
	test.Helper()
	remoteDir := test.TempDir()
	runGit(test, remoteDir, "init", "--bare", "-b", "main")

	dir := test.TempDir()
	runGit(test, dir, "init", "-b", "main")
	runGit(test, dir, "config", "user.name", "Test Author")
	runGit(test, dir, "config", "user.email", "test@example.com")
	commitFile(test, dir, "base")
	runGit(test, dir, "remote", "add", "origin", remoteDir)
	runGit(test, dir, "push", "-u", "origin", "main")
	commitFile(test, dir, "local")

	app := New()
	if _, err := app.OpenRepository(dir); err != nil {
		test.Fatalf("OpenRepository: %v", err)
	}
	return dir, app
}

// editRequest returns a message-only edit of hash.
func editRequest(hash string) EditRequest {
	return EditRequest{
		Hash:        hash,
		Message:     "edited\n",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}
}

// TestUpdateCommit_RejectsCommitPushedAfterOpen verifies that a commit pushed
// from a terminal while the repository is open can no longer be edited, and
// that the app's commit log then shows it as pushed.
func TestUpdateCommit_RejectsCommitPushedAfterOpen(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")

	runGit(test, dir, "push")

	_, err := app.UpdateCommit(editRequest(local))
	if !errors.Is(err, gitpkg.ErrCommitNotUnpushed) {
		test.Fatalf("UpdateCommit error = %v, want ErrCommitNotUnpushed", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != local {
		test.Errorf("HEAD = %s, want it unchanged at %s", got, local)
	}

	commits, err := app.GetCommitLog()
	if err != nil {
		test.Fatalf("GetCommitLog: %v", err)
	}
	if commits[0].Hash != local || commits[0].IsUnpushed {
		test.Errorf("commits[0] = %+v, want %s marked as pushed", commits[0], local)
	}
}

// TestUpdateCommit_EditsUnpushedCommit is the control case: the same commit
// can be edited while it is still unpushed.
func TestUpdateCommit_EditsUnpushedCommit(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")

	if _, err := app.UpdateCommit(editRequest(local)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	if got := runGit(test, dir, "log", "-1", "--format=%s"); got != "edited" {
		test.Errorf("HEAD message = %q, want %q", got, "edited")
	}
}

// TestUndo_RejectsAfterEditedCommitIsPushed verifies that undo is refused
// once the rewritten commit has been pushed, since moving the branch back
// would discard published history.
func TestUndo_RejectsAfterEditedCommitIsPushed(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")

	if _, err := app.UpdateCommit(editRequest(local)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	edited := runGit(test, dir, "rev-parse", "HEAD")
	runGit(test, dir, "push")

	_, err := app.UndoLastOperation()
	if !errors.Is(err, gitpkg.ErrUndoPushed) {
		test.Fatalf("UndoLastOperation error = %v, want ErrUndoPushed", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != edited {
		test.Errorf("HEAD = %s, want it unchanged at %s", got, edited)
	}
	if app.CanUndo() {
		test.Error("CanUndo should be false once undo can never succeed")
	}
}

// TestUndo_RestoresUnpushedEdit is the control case: an edit that has not
// been pushed can be undone.
func TestUndo_RestoresUnpushedEdit(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")

	if _, err := app.UpdateCommit(editRequest(local)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	if _, err := app.UndoLastOperation(); err != nil {
		test.Fatalf("UndoLastOperation: %v", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != local {
		test.Errorf("HEAD = %s, want %s", got, local)
	}
}

// TestUpdateCommit_MovesBranchesAndUndoRestoresThem verifies that a branch
// reported by GetAffectedRefs can be moved with the edit, and that undo moves
// it back together with the edited branch.
func TestUpdateCommit_MovesBranchesAndUndoRestoresThem(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")
	runGit(test, dir, "branch", "same-tip")

	refs, err := app.GetAffectedRefs([]string{local})
	if err != nil {
		test.Fatalf("GetAffectedRefs: %v", err)
	}
	if len(refs) != 1 || refs[0] != (AffectedRef{Name: "same-tip", Kind: "branch"}) {
		test.Fatalf("GetAffectedRefs = %+v, want [same-tip branch]", refs)
	}

	req := editRequest(local)
	req.MoveBranches = []string{"same-tip"}
	result, err := app.UpdateCommit(req)
	if err != nil || !result.Success {
		test.Fatalf("UpdateCommit = %+v, %v", result, err)
	}
	edited := runGit(test, dir, "rev-parse", "main")
	if got := runGit(test, dir, "rev-parse", "same-tip"); got != edited {
		test.Errorf("same-tip = %s, want %s", got, edited)
	}

	result, err = app.UndoLastOperation()
	if err != nil || !result.Success {
		test.Fatalf("UndoLastOperation = %+v, %v", result, err)
	}
	for _, branch := range []string{"main", "same-tip"} {
		if got := runGit(test, dir, "rev-parse", branch); got != local {
			test.Errorf("%s = %s after undo, want %s", branch, got, local)
		}
	}
}

// TestUpdateCommit_KeepsStashWhenWorktreeIsClean verifies that an edit in a
// clean worktree leaves an existing stash alone. The committed executable file
// reproduces the Windows case where go-git reported the clean worktree as
// dirty, which made the old auto-stash run a no-op `git stash` and then pop
// the user's unrelated stash.
func TestUpdateCommit_KeepsStashWhenWorktreeIsClean(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)

	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}
	runGit(test, dir, "add", "run.sh")
	runGit(test, dir, "update-index", "--chmod=+x", "run.sh")
	runGit(test, dir, "commit", "-m", "script")

	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("stashed work"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}
	runGit(test, dir, "stash")
	stashBefore := runGit(test, dir, "stash", "list")

	head := runGit(test, dir, "rev-parse", "HEAD")
	if _, err := app.UpdateCommit(editRequest(head)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}

	if got := runGit(test, dir, "stash", "list"); got != stashBefore {
		test.Errorf("stash list = %q, want %q", got, stashBefore)
	}
	if got := runGit(test, dir, "status", "--porcelain"); got != "" {
		test.Errorf("status = %q, want a clean worktree", got)
	}
}

// TestUpdateCommit_KeepsStagedAndUnstagedChanges verifies that an edit leaves
// uncommitted work exactly as it was: staged changes stay staged and unstaged
// changes stay unstaged. Rewrites only change metadata, so nothing needs to be
// stashed.
func TestUpdateCommit_KeepsStagedAndUnstagedChanges(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)

	if err := os.WriteFile(filepath.Join(dir, "local.txt"), []byte("staged"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}
	runGit(test, dir, "add", "local.txt")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("unstaged"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}
	statusBefore := runGit(test, dir, "status", "--porcelain")

	head := runGit(test, dir, "rev-parse", "HEAD")
	if _, err := app.UpdateCommit(editRequest(head)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}

	if got := runGit(test, dir, "status", "--porcelain"); got != statusBefore {
		test.Errorf("status = %q, want %q", got, statusBefore)
	}
	if got := runGit(test, dir, "stash", "list"); got != "" {
		test.Errorf("stash list = %q, want it empty", got)
	}
}

// TestUpdateCommit_RejectsInvalidAuthor verifies that the bound method refuses
// an author that would produce a malformed commit.
func TestUpdateCommit_RejectsInvalidAuthor(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	head := runGit(test, dir, "rev-parse", "HEAD")

	req := editRequest(head)
	req.AuthorName = "Name <with brackets>"
	if _, err := app.UpdateCommit(req); !errors.Is(err, gitpkg.ErrInvalidIdentity) {
		test.Fatalf("expected ErrInvalidIdentity, got %v", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != head {
		test.Errorf("HEAD moved from %s to %s", head, got)
	}
}

// TestShiftCommitDates_ShiftsAndUndoRestores verifies that several unpushed
// commits are shifted in one rewrite, keeping their committer dates, and
// that a single undo restores the original branch tip.
func TestShiftCommitDates_ShiftsAndUndoRestores(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	commitFile(test, dir, "second")
	if _, err := app.ReloadRepository(); err != nil {
		test.Fatalf("ReloadRepository: %v", err)
	}
	before := runGit(test, dir, "rev-parse", "HEAD")
	datesBefore := runGit(test, dir, "log", "-2", "--format=%at %ct")

	result, err := app.ShiftCommitDates(ShiftRequest{
		Hashes:  []string{before, runGit(test, dir, "rev-parse", "HEAD~1")},
		Minutes: 90,
	})
	if err != nil || !result.Success || result.Message != "2 commits shifted" {
		test.Fatalf("ShiftCommitDates = %+v, %v", result, err)
	}

	beforeLines := strings.Split(datesBefore, "\n")
	afterLines := strings.Split(runGit(test, dir, "log", "-2", "--format=%at %ct"), "\n")
	for i := range beforeLines {
		var authorBefore, committerBefore, authorAfter, committerAfter int64
		fmt.Sscan(beforeLines[i], &authorBefore, &committerBefore)
		fmt.Sscan(afterLines[i], &authorAfter, &committerAfter)
		if authorAfter-authorBefore != 90*60 {
			test.Errorf("commit %d author date moved by %ds, want %ds", i, authorAfter-authorBefore, 90*60)
		}
		if committerAfter != committerBefore {
			test.Errorf("commit %d committer date changed: %d -> %d", i, committerBefore, committerAfter)
		}
	}

	if result, err := app.UndoLastOperation(); err != nil || !result.Success {
		test.Fatalf("UndoLastOperation = %+v, %v", result, err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != before {
		test.Errorf("HEAD = %s after undo, want %s", got, before)
	}
}

// TestShiftCommitDates_RejectsPushedCommit verifies that including a pushed
// commit refuses the whole shift.
func TestShiftCommitDates_RejectsPushedCommit(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	head := runGit(test, dir, "rev-parse", "HEAD")

	_, err := app.ShiftCommitDates(ShiftRequest{
		Hashes:  []string{head, runGit(test, dir, "rev-parse", "HEAD~1")},
		Minutes: 60,
	})
	if !errors.Is(err, gitpkg.ErrCommitNotUnpushed) {
		test.Fatalf("expected ErrCommitNotUnpushed, got %v", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != head {
		test.Errorf("HEAD moved from %s to %s", head, got)
	}
}
