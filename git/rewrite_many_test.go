package git_test

import (
	"errors"
	"strings"
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

// TestEditCommits_KeepsOffsetAndOptionallyShiftsCommitter verifies that a
// shift keeps each commit's own time zone offset, and moves the committer
// date only when asked.
func TestEditCommits_KeepsOffsetAndOptionallyShiftsCommitter(test *testing.T) {
	for _, shiftCommitter := range []bool{false, true} {
		dir := test.TempDir()
		gitCmd := initRepo(test, dir)
		addCommit(test, dir, "first", gitCmd)
		gitCmd("commit", "--amend", "--no-edit", "--date=2024-01-01T12:00:00+05:30")

		opts := git.BulkEditOptions{Shift: -90 * time.Minute, ShiftCommitter: shiftCommitter}
		if err := git.EditCommits(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, opts); err != nil {
			test.Fatalf("git.EditCommits: %v", err)
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

// TestEditCommits_RejectsNoChange verifies that a zero shift without a new
// author is an error rather than a rewrite that only changes hashes.
func TestEditCommits_RejectsNoChange(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)

	err := git.EditCommits(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, git.BulkEditOptions{})
	if err == nil {
		test.Fatal("expected an error for an edit that changes nothing")
	}
}

// TestEditCommits_SetsAuthorKeepingDatesAndCommitter verifies that setting
// the author on several commits replaces only the author name and email,
// keeping the dates, messages and committers.
func TestEditCommits_SetsAuthorKeepingDatesAndCommitter(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	format := "--format=%an <%ae> %aI | %cn <%ce> %cI | %s"
	before := gitOutputFromDir(test, dir, "git", "log", format)

	opts := git.BulkEditOptions{SetAuthor: true, AuthorName: "New Name", AuthorEmail: "new@example.com"}
	hashes := []plumbing.Hash{revHash(test, dir, "HEAD"), revHash(test, dir, "HEAD~1")}
	if err := git.EditCommits(mustOpen(test, dir), hashes, opts); err != nil {
		test.Fatalf("git.EditCommits: %v", err)
	}

	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(gitOutputFromDir(test, dir, "git", "log", format), "\n")
	for i, line := range afterLines {
		want := beforeLines[i]
		if i < 2 {
			_, rest, _ := strings.Cut(want, "> ")
			want = "New Name <new@example.com> " + rest
		}
		if line != want {
			test.Errorf("commit %d = %q, want %q", i, line, want)
		}
	}
}

// TestEditCommits_ShiftsAndSetsAuthorTogether verifies that a shift and a new
// author are applied in the same rewrite.
func TestEditCommits_ShiftsAndSetsAuthorTogether(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)

	opts := git.BulkEditOptions{Shift: time.Hour, SetAuthor: true, AuthorName: "New Name", AuthorEmail: "new@example.com"}
	if err := git.EditCommits(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, opts); err != nil {
		test.Fatalf("git.EditCommits: %v", err)
	}

	want := "New Name <new@example.com> " + testCommitDate.Add(time.Hour).Format(time.RFC3339)
	if got := gitOutputFromDir(test, dir, "git", "log", "-1", "--format=%an <%ae> %aI"); got != want {
		test.Errorf("author = %q, want %q", got, want)
	}
}

// TestEditCommits_SetsCommitterKeepingEverythingElse verifies that a new
// committer on its own is a valid edit and changes only the committer name
// and email of the selected commits.
func TestEditCommits_SetsCommitterKeepingEverythingElse(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	format := "--format=%an <%ae> %aI | %cn <%ce> %cI | %s"
	before := gitOutputFromDir(test, dir, "git", "log", format)

	opts := git.BulkEditOptions{Committer: git.SetCommitter, CommitterName: "New Committer", CommitterEmail: "c@example.com"}
	hashes := []plumbing.Hash{revHash(test, dir, "HEAD"), revHash(test, dir, "HEAD~1")}
	if err := git.EditCommits(mustOpen(test, dir), hashes, opts); err != nil {
		test.Fatalf("git.EditCommits: %v", err)
	}

	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(gitOutputFromDir(test, dir, "git", "log", format), "\n")
	for i, line := range afterLines {
		want := beforeLines[i]
		if i < 2 {
			want = strings.Replace(want, "| Test Author <test@example.com>", "| New Committer <c@example.com>", 1)
		}
		if line != want {
			test.Errorf("commit %d = %q, want %q", i, line, want)
		}
	}
}

// TestEditCommits_CommitterFromAuthor verifies that CommitterFromAuthor gives
// each commit a committer matching its own author, after any author change.
func TestEditCommits_CommitterFromAuthor(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	gitCmd("commit", "--allow-empty", "-m", "other author", "--author", "Other Person <other@example.com>")
	addCommit(test, dir, "second", gitCmd)
	hashes := []plumbing.Hash{revHash(test, dir, "HEAD"), revHash(test, dir, "HEAD~1")}

	opts := git.BulkEditOptions{Committer: git.CommitterFromAuthor}
	if err := git.EditCommits(mustOpen(test, dir), hashes, opts); err != nil {
		test.Fatalf("git.EditCommits: %v", err)
	}
	want := "Test Author <test@example.com>\nOther Person <other@example.com>\nTest Author <test@example.com>"
	if got := gitOutputFromDir(test, dir, "git", "log", "--format=%cn <%ce>"); got != want {
		test.Errorf("committers = %q, want %q", got, want)
	}

	hashes = []plumbing.Hash{revHash(test, dir, "HEAD"), revHash(test, dir, "HEAD~1")}
	opts = git.BulkEditOptions{SetAuthor: true, AuthorName: "New Name", AuthorEmail: "new@example.com", Committer: git.CommitterFromAuthor}
	if err := git.EditCommits(mustOpen(test, dir), hashes, opts); err != nil {
		test.Fatalf("git.EditCommits: %v", err)
	}
	want = "New Name <new@example.com>\nNew Name <new@example.com>\nTest Author <test@example.com>"
	if got := gitOutputFromDir(test, dir, "git", "log", "--format=%cn <%ce>"); got != want {
		test.Errorf("committers = %q, want %q", got, want)
	}
}

// TestEditCommits_RejectsInvalidIdentity verifies that an author or committer
// that would produce a malformed commit is refused before anything is
// rewritten.
func TestEditCommits_RejectsInvalidIdentity(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)
	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")

	for _, opts := range []git.BulkEditOptions{
		{SetAuthor: true, AuthorName: " ", AuthorEmail: "new@example.com"},
		{SetAuthor: true, AuthorName: "New Name", AuthorEmail: "<new@example.com>"},
		{Committer: git.SetCommitter, CommitterName: "", CommitterEmail: "c@example.com"},
		{Committer: git.SetCommitter, CommitterName: "Two\nLines", CommitterEmail: "c@example.com"},
	} {
		err := git.EditCommits(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, opts)
		if !errors.Is(err, git.ErrInvalidIdentity) {
			test.Errorf("%+v: expected ErrInvalidIdentity, got %v", opts, err)
		}
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"); got != before {
		test.Errorf("branch moved: %s -> %s", before, got)
	}
}

// TestConfiguredIdentity_ReadsRepositoryConfig verifies that the identity
// comes from the repository's user.name / user.email.
func TestConfiguredIdentity_ReadsRepositoryConfig(test *testing.T) {
	test.Setenv("GIT_AUTHOR_NAME", "")
	test.Setenv("GIT_AUTHOR_EMAIL", "")
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "only", gitCmd)
	gitCmd("config", "user.name", "Configured Name")
	gitCmd("config", "user.email", "configured@example.com")

	identity, err := git.ConfiguredIdentity(mustOpen(test, dir))
	if err != nil {
		test.Fatalf("git.ConfiguredIdentity: %v", err)
	}
	want := git.Identity{Name: "Configured Name", Email: "configured@example.com"}
	if identity != want {
		test.Errorf("identity = %+v, want %+v", identity, want)
	}
}
