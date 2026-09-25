package git_test

import (
	"errors"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// TestResetBranch_RestoresPreviousTip verifies that an amend can be undone by
// moving the branch back to the pre-rewrite commit.
func TestResetBranch_RestoresPreviousTip(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	beforeHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"))

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, mustOpen(test, dir), opts)

	afterHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"))
	if afterHash == beforeHash {
		test.Fatalf("amend did not change HEAD")
	}

	err := git.ResetBranch(mustOpen(test, dir), plumbing.NewBranchReferenceName("main"), afterHash, beforeHash)
	if err != nil {
		test.Fatalf("git.ResetBranch: %v", err)
	}

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if entries[0].Hash != beforeHash {
		test.Errorf("HEAD = %s, want %s", entries[0].Hash, beforeHash)
	}
	if entries[0].Message != "original commit" {
		test.Errorf("Message = %q, want %q", entries[0].Message, "original commit")
	}
}

// TestResetBranch_RejectsWhenBranchMoved verifies that undo is refused once a
// new commit has been made on top of the rewritten history.
func TestResetBranch_RejectsWhenBranchMoved(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	beforeHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"))

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, mustOpen(test, dir), opts)

	afterHash := plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"))
	addCommit(test, dir, "newer commit", gitCmd)
	newestHash := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")

	err := git.ResetBranch(mustOpen(test, dir), plumbing.NewBranchReferenceName("main"), afterHash, beforeHash)
	if !errors.Is(err, git.ErrBranchMoved) {
		test.Fatalf("expected ErrBranchMoved, got %v", err)
	}

	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"); got != newestHash {
		test.Errorf("HEAD = %s, want unchanged %s", got, newestHash)
	}
}
