package git_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// mustAmend calls AmendCommit and fails the test on error.
func mustAmend(test *testing.T, repoState *git.RepoState, opts git.AmendOptions) {
	test.Helper()
	if err := git.AmendCommit(repoState, opts); err != nil {
		test.Fatalf("git.AmendCommit: %v", err)
	}
}

// gitOutputFromDir runs a git command in dir and returns the trimmed stdout.
func gitOutputFromDir(test *testing.T, dir string, args ...string) string {
	test.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		test.Fatalf("git %v: %v", args[1:], err)
	}
	return strings.TrimSpace(string(out))
}

// baseAmendOpts returns a valid AmendOptions built from the original commit so
// that tests only need to override the single field under test.
func baseAmendOpts() git.AmendOptions {
	return git.AmendOptions{
		Message:     "original commit\n",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Date:        testCommitDate,
	}
}

// TestAmendCommit_UpdatesMessage verifies that the commit message is replaced.
func TestAmendCommit_UpdatesMessage(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	repoState := mustOpen(test, dir)

	opts := baseAmendOpts()
	opts.Message = "amended message\n"
	mustAmend(test, repoState, opts)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if entries[0].Message != "amended message" {
		test.Errorf("Message = %q, want %q", entries[0].Message, "amended message")
	}
}

// TestAmendCommit_UpdatesAuthor verifies that author name and email are replaced.
func TestAmendCommit_UpdatesAuthor(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	repoState := mustOpen(test, dir)

	opts := baseAmendOpts()
	opts.AuthorName = "New Author"
	opts.AuthorEmail = "new@example.com"
	mustAmend(test, repoState, opts)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if entries[0].AuthorName != "New Author" {
		test.Errorf("AuthorName = %q, want %q", entries[0].AuthorName, "New Author")
	}

	// CommitEntry has no AuthorEmail field; verify via native git.
	gotEmail := gitOutputFromDir(test, dir, "git", "log", "--format=%ae", "-1")
	if gotEmail != "new@example.com" {
		test.Errorf("AuthorEmail = %q, want %q", gotEmail, "new@example.com")
	}
}

// TestAmendCommit_UpdatesDate verifies that the author date is replaced.
func TestAmendCommit_UpdatesDate(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	repoState := mustOpen(test, dir)

	newDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	opts := baseAmendOpts()
	opts.Date = newDate
	mustAmend(test, repoState, opts)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if !entries[0].Date.Equal(newDate) {
		test.Errorf("Date = %v, want %v", entries[0].Date, newDate)
	}
}

// TestAmendCommit_KeepsTree verifies that the file tree is unchanged after amend.
func TestAmendCommit_KeepsTree(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	repoState := mustOpen(test, dir)
	beforeTree := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^{tree}")

	mustAmend(test, repoState, baseAmendOpts())

	afterTree := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^{tree}")
	if beforeTree != afterTree {
		test.Errorf("tree hash changed after amend: %s -> %s", beforeTree, afterTree)
	}
}

// TestAmendCommit_KeepsParents verifies that the parent chain is unchanged after amend.
func TestAmendCommit_KeepsParents(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	repoState := mustOpen(test, dir)
	beforeParent := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^1")

	mustAmend(test, repoState, baseAmendOpts())

	afterParent := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^1")
	if beforeParent != afterParent {
		test.Errorf("parent hash changed after amend: %s -> %s", beforeParent, afterParent)
	}
}

// mustRebaseRewrite calls RebaseRewrite and fails the test on error.
func mustRebaseRewrite(test *testing.T, repoState *git.RepoState, targetHash plumbing.Hash, opts git.AmendOptions) {
	test.Helper()
	if err := git.RebaseRewrite(repoState, targetHash, opts); err != nil {
		test.Fatalf("git.RebaseRewrite: %v", err)
	}
}

// TestAmendCommit_RejectsPushedCommit verifies that amending a pushed commit
// returns ErrCommitNotUnpushed.
func TestAmendCommit_RejectsPushedCommit(test *testing.T) {
	remoteDir := makeRemote(test)

	localDir := test.TempDir()
	gitCmd := initRepo(test, localDir)
	addCommit(test, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")

	repoState := mustOpen(test, localDir)

	opts := baseAmendOpts()
	opts.Message = "should not work\n"
	err := git.AmendCommit(repoState, opts)

	if !errors.Is(err, git.ErrCommitNotUnpushed) {
		test.Fatalf("expected ErrCommitNotUnpushed, got %v", err)
	}
}

// TestRebaseRewrite_UpdatesTargetMessage verifies that the target commit's
// message is replaced while the commit above it retains its original message.
func TestRebaseRewrite_UpdatesTargetMessage(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)
	addCommit(test, dir, "third", gitCmd)

	targetHashStr := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~1")
	targetHash := plumbing.NewHash(targetHashStr)

	opts := baseAmendOpts()
	opts.Message = "amended second\n"
	mustRebaseRewrite(test, mustOpen(test, dir), targetHash, opts)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if len(entries) != 3 {
		test.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Message != "third" {
		test.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "third")
	}
	if entries[1].Message != "amended second" {
		test.Errorf("entries[1].Message = %q, want %q", entries[1].Message, "amended second")
	}
}

// TestRebaseRewrite_KeepsNewerCommitContent verifies that the commit above the
// target retains its original file tree after the chain is rebuilt.
func TestRebaseRewrite_KeepsNewerCommitContent(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)
	addCommit(test, dir, "third", gitCmd)

	beforeTree := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^{tree}")

	targetHashStr := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~1")
	targetHash := plumbing.NewHash(targetHashStr)

	mustRebaseRewrite(test, mustOpen(test, dir), targetHash, baseAmendOpts())

	afterTree := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^{tree}")
	if beforeTree != afterTree {
		test.Errorf("top commit tree changed after rebase: %s -> %s", beforeTree, afterTree)
	}
}

// TestRebaseRewrite_PreservesUnchangedParent verifies that the commit below the
// target (which is not part of the rewritten chain) keeps its original hash.
func TestRebaseRewrite_PreservesUnchangedParent(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)
	addCommit(test, dir, "third", gitCmd)

	origFirstHash := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~2")

	targetHashStr := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~1")
	targetHash := plumbing.NewHash(targetHashStr)

	mustRebaseRewrite(test, mustOpen(test, dir), targetHash, baseAmendOpts())

	aftFirstHash := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~2")
	if origFirstHash != aftFirstHash {
		test.Errorf("parent below target changed: %s -> %s", origFirstHash, aftFirstHash)
	}
}

// TestRebaseRewrite_TargetIsHead verifies that targeting HEAD directly works
// equivalently to AmendCommit.
func TestRebaseRewrite_TargetIsHead(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original", gitCmd)

	headHashStr := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	headHash := plumbing.NewHash(headHashStr)

	opts := baseAmendOpts()
	opts.Message = "rewritten\n"
	mustRebaseRewrite(test, mustOpen(test, dir), headHash, opts)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)
	if entries[0].Message != "rewritten" {
		test.Errorf("Message = %q, want %q", entries[0].Message, "rewritten")
	}
}

// TestRebaseRewrite_RejectsPushedCommit verifies that targeting a pushed commit
// returns ErrCommitNotUnpushed.
func TestRebaseRewrite_RejectsPushedCommit(test *testing.T) {
	remoteDir := makeRemote(test)

	localDir := test.TempDir()
	gitCmd := initRepo(test, localDir)
	addCommit(test, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(test, localDir, "unpushed", gitCmd)

	pushedHashStr := gitOutputFromDir(test, localDir, "git", "rev-parse", "HEAD~1")
	pushedHash := plumbing.NewHash(pushedHashStr)

	err := git.RebaseRewrite(mustOpen(test, localDir), pushedHash, baseAmendOpts())
	if !errors.Is(err, git.ErrCommitNotUnpushed) {
		test.Fatalf("expected ErrCommitNotUnpushed, got %v", err)
	}
}
