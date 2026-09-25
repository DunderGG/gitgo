package git_test

import (
	"errors"
	"testing"
	"time"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// shiftEdit returns a CommitEdit that moves both dates of a commit by d and
// keeps everything else.
func shiftEdit(d time.Duration) git.CommitEdit {
	return func(original *object.Commit) (object.Signature, object.Signature, string) {
		author, committer := original.Author, original.Committer
		author.When = author.When.Add(d)
		committer.When = committer.When.Add(d)
		return author, committer, original.Message
	}
}

// revHash resolves rev in dir to a hash.
func revHash(test *testing.T, dir, rev string) plumbing.Hash {
	test.Helper()
	return plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", rev))
}

// TestRewriteCommits_EditsSeveralCommitsInOnePass verifies that two
// non-adjacent commits are edited, the commit between them only gets a new
// parent, and the branch moves once with a single reflog entry.
func TestRewriteCommits_EditsSeveralCommitsInOnePass(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)
	addCommit(test, dir, "third", gitCmd)

	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	base := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~3")
	reflogEntries := reflogCount(test, dir, "main")

	edits := map[plumbing.Hash]git.CommitEdit{
		revHash(test, dir, "HEAD"):   shiftEdit(time.Hour),
		revHash(test, dir, "HEAD~2"): shiftEdit(time.Hour),
	}
	if err := git.RewriteCommits(mustOpen(test, dir), edits, nil); err != nil {
		test.Fatalf("git.RewriteCommits: %v", err)
	}

	shifted := testCommitDate.Add(time.Hour).Format(time.RFC3339)
	original := testCommitDate.Format(time.RFC3339)
	for rev, want := range map[string]string{"HEAD": shifted, "HEAD~1": original, "HEAD~2": shifted, "HEAD~3": original} {
		dates := gitOutputFromDir(test, dir, "git", "log", "-1", "--format=%aI %cI", rev)
		if dates != want+" "+want {
			test.Errorf("%s author/committer dates = %q, want both %s", rev, dates, want)
		}
	}
	if got := gitOutputFromDir(test, dir, "git", "log", "--format=%s", "HEAD"); got != "third\nsecond\nfirst\nbase" {
		test.Errorf("messages = %q, want unchanged", got)
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~3"); got != base {
		test.Errorf("commit below the oldest edit changed: %s -> %s", base, got)
	}

	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "main@{1}"); got != before {
		test.Errorf("main@{1} = %s, want %s", got, before)
	}
	if got := reflogCount(test, dir, "main"); got != reflogEntries+1 {
		test.Errorf("main reflog entries = %d, want %d", got, reflogEntries+1)
	}
	if got := reflogSubject(test, dir, "main"); got != "gitgo: edit 2 commits" {
		test.Errorf("reflog message = %q, want %q", got, "gitgo: edit 2 commits")
	}
}

// TestRewriteCommits_RejectsWhenOneCommitIsPushed verifies that the whole
// rewrite is refused, and the branch left alone, when any commit is pushed.
func TestRewriteCommits_RejectsWhenOneCommitIsPushed(test *testing.T) {
	remoteDir := makeRemote(test)

	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(test, dir, "unpushed", gitCmd)

	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	edits := map[plumbing.Hash]git.CommitEdit{
		revHash(test, dir, "HEAD"):   shiftEdit(time.Hour),
		revHash(test, dir, "HEAD~1"): shiftEdit(time.Hour),
	}
	err := git.RewriteCommits(mustOpen(test, dir), edits, nil)
	if !errors.Is(err, git.ErrCommitNotUnpushed) {
		test.Fatalf("expected ErrCommitNotUnpushed, got %v", err)
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"); got != before {
		test.Errorf("branch moved: %s -> %s", before, got)
	}
}

// TestRewriteCommits_RejectsCommitOffFirstParentChain verifies that a commit
// that is not on the branch's first-parent chain (here: on another branch)
// fails the whole rewrite and leaves the branch alone.
func TestRewriteCommits_RejectsCommitOffFirstParentChain(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	gitCmd("checkout", "-b", "other")
	addCommit(test, dir, "elsewhere", gitCmd)
	gitCmd("checkout", "main")
	addCommit(test, dir, "tip", gitCmd)

	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	edits := map[plumbing.Hash]git.CommitEdit{
		revHash(test, dir, "HEAD"):  shiftEdit(time.Hour),
		revHash(test, dir, "other"): shiftEdit(time.Hour),
	}
	repoState := mustOpen(test, dir)
	// The other branch's commit is unpushed too (there is no remote), so the
	// rewrite must be refused by the chain walk, not the pushed check.
	repoState.UnpushedHashes[revHash(test, dir, "other")] = true
	if err := git.RewriteCommits(repoState, edits, nil); err == nil {
		test.Fatal("expected an error for a commit off the first-parent chain")
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"); got != before {
		test.Errorf("branch moved: %s -> %s", before, got)
	}
}

// TestRewriteCommits_RejectsNoEdits verifies that an empty edit set is an
// error rather than a no-op rewrite.
func TestRewriteCommits_RejectsNoEdits(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)

	if err := git.RewriteCommits(mustOpen(test, dir), nil, nil); err == nil {
		test.Fatal("expected an error for no edits")
	}
}

// TestShiftDates_KeepsOffsetAndOptionallyShiftsCommitter verifies that a
// shift keeps each commit's own time zone offset, and moves the committer
// date only when asked.
func TestShiftDates_KeepsOffsetAndOptionallyShiftsCommitter(test *testing.T) {
	for _, shiftCommitter := range []bool{false, true} {
		dir := test.TempDir()
		gitCmd := initRepo(test, dir)
		addCommit(test, dir, "first", gitCmd)
		gitCmd("commit", "--amend", "--no-edit", "--date=2024-01-01T12:00:00+05:30")

		opts := git.ShiftOptions{Shift: -90 * time.Minute, ShiftCommitter: shiftCommitter}
		if err := git.ShiftDates(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, opts); err != nil {
			test.Fatalf("git.ShiftDates: %v", err)
		}

		wantCommitter := testCommitDate.Format(time.RFC3339)
		if shiftCommitter {
			wantCommitter = testCommitDate.Add(-90 * time.Minute).Format(time.RFC3339)
		}
		want := "2024-01-01T10:30:00+05:30 " + wantCommitter
		if got := gitOutputFromDir(test, dir, "git", "log", "-1", "--format=%aI %cI"); got != want {
			test.Errorf("shiftCommitter=%v: author/committer dates = %q, want %q", shiftCommitter, got, want)
		}
	}
}

// TestShiftDates_RejectsZeroShift verifies that a zero shift is an error
// rather than a rewrite that only changes hashes.
func TestShiftDates_RejectsZeroShift(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)

	err := git.ShiftDates(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, git.ShiftOptions{})
	if err == nil {
		test.Fatal("expected an error for a zero shift")
	}
}
