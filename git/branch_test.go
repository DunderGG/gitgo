package git_test

import (
	"errors"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// setupFeatureBranch creates a repo where main has one commit and "feature"
// has two more on top, with main checked out. It returns the repo directory.
func setupFeatureBranch(test *testing.T) (string, func(...string)) {
	test.Helper()
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	gitCmd("switch", "-c", "feature")
	addCommit(test, dir, "feature one", gitCmd)
	addCommit(test, dir, "feature two", gitCmd)
	gitCmd("switch", "main")
	return dir, gitCmd
}

// mustOpenBranch opens branch in the repository at dir and fails the test on error.
func mustOpenBranch(test *testing.T, dir, branch string) *git.RepoState {
	test.Helper()
	repoState, err := git.OpenBranch(dir, branch)
	if err != nil {
		test.Fatalf("git.OpenBranch(%q, %q): %v", dir, branch, err)
	}
	return repoState
}

// TestListBranches_ReturnsSortedLocalBranches verifies that every local branch
// is listed in alphabetical order.
func TestListBranches_ReturnsSortedLocalBranches(test *testing.T) {
	dir, gitCmd := setupFeatureBranch(test)
	gitCmd("branch", "another")

	branches, err := git.ListBranches(mustOpen(test, dir))
	if err != nil {
		test.Fatalf("git.ListBranches: %v", err)
	}

	want := []string{"another", "feature", "main"}
	if len(branches) != len(want) {
		test.Fatalf("branches = %v, want %v", branches, want)
	}
	for index := range want {
		if branches[index] != want[index] {
			test.Errorf("branches[%d] = %q, want %q", index, branches[index], want[index])
		}
	}
}

// TestOpenBranch_EmptyNameUsesCheckedOutBranch verifies that an empty branch
// name behaves like Open.
func TestOpenBranch_EmptyNameUsesCheckedOutBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)

	repoState := mustOpenBranch(test, dir, "")
	if repoState.Branch != "main" {
		test.Errorf("Branch = %q, want %q", repoState.Branch, "main")
	}
	if !repoState.IsCheckedOut {
		test.Errorf("IsCheckedOut = false, want true")
	}
}

// TestOpenBranch_TargetsNonCheckedOutBranch verifies that the log of a branch
// that is not checked out can be read.
func TestOpenBranch_TargetsNonCheckedOutBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)

	repoState := mustOpenBranch(test, dir, "feature")
	if repoState.Branch != "feature" {
		test.Errorf("Branch = %q, want %q", repoState.Branch, "feature")
	}
	if repoState.IsCheckedOut {
		test.Errorf("IsCheckedOut = true, want false")
	}

	entries := mustLog(test, repoState, noLogLimit)
	if len(entries) != 3 {
		test.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Message != "feature two" {
		test.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "feature two")
	}
}

// TestOpenBranch_UnknownBranch verifies that a missing branch is reported
// with ErrBranchNotFound.
func TestOpenBranch_UnknownBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)

	_, err := git.OpenBranch(dir, "does-not-exist")
	if !errors.Is(err, git.ErrBranchNotFound) {
		test.Fatalf("expected ErrBranchNotFound, got %v", err)
	}
}

// TestAmendCommit_NonCheckedOutBranch verifies that amending the tip of a
// branch that is not checked out moves only that branch and leaves HEAD and
// the working tree alone.
func TestAmendCommit_NonCheckedOutBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)
	mainBefore := gitOutputFromDir(test, dir, "git", "rev-parse", "main")

	opts := baseAmendOpts()
	opts.Message = "amended feature two\n"
	mustAmend(test, mustOpenBranch(test, dir, "feature"), opts)

	entries := mustLog(test, mustOpenBranch(test, dir, "feature"), noLogLimit)
	if entries[0].Message != "amended feature two" {
		test.Errorf("feature tip message = %q, want %q", entries[0].Message, "amended feature two")
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "main"); got != mainBefore {
		test.Errorf("main = %s, want unchanged %s", got, mainBefore)
	}
	if got := gitOutputFromDir(test, dir, "git", "symbolic-ref", "--short", "HEAD"); got != "main" {
		test.Errorf("HEAD branch = %q, want %q", got, "main")
	}
	if got := gitOutputFromDir(test, dir, "git", "status", "--porcelain"); got != "" {
		test.Errorf("working tree changed:\n%s", got)
	}
}

// TestRebaseRewrite_NonCheckedOutBranch verifies that an older commit on a
// branch that is not checked out can be rewritten.
func TestRebaseRewrite_NonCheckedOutBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)
	mainBefore := gitOutputFromDir(test, dir, "git", "rev-parse", "main")
	targetHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "feature~1"))

	opts := baseAmendOpts()
	opts.Message = "amended feature one\n"
	mustRebaseRewrite(test, mustOpenBranch(test, dir, "feature"), targetHash, opts)

	entries := mustLog(test, mustOpenBranch(test, dir, "feature"), noLogLimit)
	if entries[0].Message != "feature two" {
		test.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "feature two")
	}
	if entries[1].Message != "amended feature one" {
		test.Errorf("entries[1].Message = %q, want %q", entries[1].Message, "amended feature one")
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "main"); got != mainBefore {
		test.Errorf("main = %s, want unchanged %s", got, mainBefore)
	}
}

// TestResetBranch_NonCheckedOutBranch verifies that a rewrite on a branch that
// is not checked out can be undone.
func TestResetBranch_NonCheckedOutBranch(test *testing.T) {
	dir, _ := setupFeatureBranch(test)
	beforeHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "feature"))

	opts := baseAmendOpts()
	opts.Message = "amended feature two\n"
	mustAmend(test, mustOpenBranch(test, dir, "feature"), opts)
	afterHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "feature"))

	err := git.ResetBranch(mustOpen(test, dir), plumbing.NewBranchReferenceName("feature"), afterHash, beforeHash)
	if err != nil {
		test.Fatalf("git.ResetBranch: %v", err)
	}

	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "feature"); got != beforeHash.String() {
		test.Errorf("feature = %s, want %s", got, beforeHash)
	}
}
