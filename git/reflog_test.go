package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// reflogCount returns the number of reflog entries for ref.
func reflogCount(test *testing.T, dir, ref string) int {
	test.Helper()
	out := gitOutputFromDir(test, dir, "git", "reflog", "show", "--format=%H", ref)
	if out == "" {
		return 0
	}
	return strings.Count(out, "\n") + 1
}

// reflogSubject returns the message of the newest reflog entry for ref.
func reflogSubject(test *testing.T, dir, ref string) string {
	test.Helper()
	return gitOutputFromDir(test, dir, "git", "reflog", "show", "-1", "--format=%gs", ref)
}

// TestReflog_AmendRecordsBranchAndHead verifies that amending the checked-out
// branch adds one entry to both the branch and HEAD reflogs, so the old tip
// can be recovered with main@{1} / HEAD@{1}.
func TestReflog_AmendRecordsBranchAndHead(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "original commit", gitCmd)

	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	branchEntries := reflogCount(test, dir, "main")
	headEntries := reflogCount(test, dir, "HEAD")

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, mustOpen(test, dir), opts)

	wantSubject := "gitgo: edit commit " + before[:7]
	for _, ref := range []string{"main", "HEAD"} {
		if got := gitOutputFromDir(test, dir, "git", "rev-parse", ref+"@{1}"); got != before {
			test.Errorf("%s@{1} = %s, want %s", ref, got, before)
		}
		if got := reflogSubject(test, dir, ref); got != wantSubject {
			test.Errorf("%s reflog message = %q, want %q", ref, got, wantSubject)
		}
	}
	if got := reflogCount(test, dir, "main"); got != branchEntries+1 {
		test.Errorf("main reflog entries = %d, want %d", got, branchEntries+1)
	}
	if got := reflogCount(test, dir, "HEAD"); got != headEntries+1 {
		test.Errorf("HEAD reflog entries = %d, want %d", got, headEntries+1)
	}

	identity := gitOutputFromDir(test, dir, "git", "reflog", "show", "-1", "--format=%gn <%ge>", "main")
	if identity != "Test Author <test@example.com>" {
		test.Errorf("reflog identity = %q, want %q", identity, "Test Author <test@example.com>")
	}
}

// TestReflog_RebaseRewriteOtherBranch verifies that rewriting a branch that is
// not checked out records the move in that branch's reflog only.
func TestReflog_RebaseRewriteOtherBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)

	before := gitOutputFromDir(test, dir, "git", "rev-parse", "feature")
	target := gitOutputFromDir(test, dir, "git", "rev-parse", "feature~1")
	headEntries := reflogCount(test, dir, "HEAD")

	opts := baseAmendOpts()
	opts.Message = "edited feature one\n"
	mustRebaseRewrite(test, mustOpenBranch(test, dir, "feature"), plumbing.NewHash(target), opts)

	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "feature@{1}"); got != before {
		test.Errorf("feature@{1} = %s, want %s", got, before)
	}
	if got, want := reflogSubject(test, dir, "feature"), "gitgo: edit commit "+target[:7]; got != want {
		test.Errorf("feature reflog message = %q, want %q", got, want)
	}
	if got := reflogCount(test, dir, "HEAD"); got != headEntries {
		test.Errorf("HEAD reflog entries = %d, want %d (HEAD points at main)", got, headEntries)
	}
}

// TestReflog_UndoRecorded verifies that undoing a rewrite is itself recorded,
// so the undone edit can still be recovered.
func TestReflog_UndoRecorded(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	before := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"))

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, mustOpen(test, dir), opts)
	after := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")

	err := git.ResetBranch(mustOpen(test, dir), plumbing.NewBranchReferenceName("main"), plumbing.NewHash(after), before)
	if err != nil {
		test.Fatalf("git.ResetBranch: %v", err)
	}

	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "main@{1}"); got != after {
		test.Errorf("main@{1} = %s, want %s", got, after)
	}
	if got := reflogSubject(test, dir, "main"); got != "gitgo: undo edit" {
		test.Errorf("reflog message = %q, want %q", got, "gitgo: undo edit")
	}
}

// TestReflog_RespectsLogAllRefUpdatesFalse verifies that no reflog file is
// created when core.logAllRefUpdates is false and none exists yet, matching
// git's behaviour.
func TestReflog_RespectsLogAllRefUpdatesFalse(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("config", "core.logAllRefUpdates", "false")
	if err := os.RemoveAll(filepath.Join(dir, ".git", "logs")); err != nil {
		test.Fatalf("RemoveAll: %v", err)
	}

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, mustOpen(test, dir), opts)

	if _, err := os.Stat(filepath.Join(dir, ".git", "logs")); !os.IsNotExist(err) {
		test.Errorf("reflog directory was created although core.logAllRefUpdates is false (err = %v)", err)
	}
}
