package git_test

import (
	"os/exec"
	"strings"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// addSignedCommit writes a commit on top of main carrying a fake signature in
// the given header (gpgsig or gpgsig-sha256), moves main to it, and returns
// its hash. The signature does not verify; only its presence matters here.
func addSignedCommit(test *testing.T, dir, header, message string) string {
	test.Helper()
	tree := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD^{tree}")
	parent := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")
	raw := "tree " + tree + "\n" +
		"parent " + parent + "\n" +
		"author Test Author <test@example.com> 1704110400 +0000\n" +
		"committer Test Author <test@example.com> 1704110400 +0000\n" +
		header + " -----BEGIN PGP SIGNATURE-----\n" +
		" \n" +
		" ZmFrZQ==\n" +
		" -----END PGP SIGNATURE-----\n" +
		"\n" + message + "\n"

	cmd := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(raw)
	out, err := cmd.Output()
	if err != nil {
		test.Fatalf("git hash-object: %v", err)
	}
	hash := strings.TrimSpace(string(out))
	gitOutputFromDir(test, dir, "git", "update-ref", "refs/heads/main", hash)
	return hash
}

// TestFindSignedCommits_ReportsSignedCommitsInChain verifies that both
// signature headers are detected, that only commits the rewrite rebuilds are
// reported (newest first), and that edited commits are marked as such.
func TestFindSignedCommits_ReportsSignedCommitsInChain(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	signedOld := addSignedCommit(test, dir, "gpgsig", "signed old")
	addCommit(test, dir, "unsigned", gitCmd)
	signedTip := addSignedCommit(test, dir, "gpgsig-sha256", "signed tip")

	signed, err := git.FindSignedCommits(mustOpen(test, dir), plumbing.NewHash(signedOld))
	if err != nil {
		test.Fatalf("git.FindSignedCommits: %v", err)
	}
	want := []git.SignedCommit{
		{Hash: plumbing.NewHash(signedTip), Subject: "signed tip", Edited: false},
		{Hash: plumbing.NewHash(signedOld), Subject: "signed old", Edited: true},
	}
	if len(signed) != len(want) || signed[0] != want[0] || signed[1] != want[1] {
		test.Errorf("signed commits = %+v, want %+v", signed, want)
	}

	// Editing the unsigned commit leaves the signed one below it alone.
	unsigned := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~1")
	signed, err = git.FindSignedCommits(mustOpen(test, dir), plumbing.NewHash(unsigned))
	if err != nil {
		test.Fatalf("git.FindSignedCommits: %v", err)
	}
	if len(signed) != 1 || signed[0].Hash != plumbing.NewHash(signedTip) {
		test.Errorf("signed commits = %+v, want only %s", signed, signedTip)
	}
}

// TestFindSignedCommits_NoneWhenUnsigned verifies that an ordinary history
// reports no signed commits.
func TestFindSignedCommits_NoneWhenUnsigned(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	first := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD~1")
	signed, err := git.FindSignedCommits(mustOpen(test, dir), plumbing.NewHash(first))
	if err != nil {
		test.Fatalf("git.FindSignedCommits: %v", err)
	}
	if len(signed) != 0 {
		test.Errorf("signed commits = %+v, want none", signed)
	}
}
