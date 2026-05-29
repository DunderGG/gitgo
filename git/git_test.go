package git_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gitgo/git"
)

const (
	// noLogLimit is passed to Log to use the package-default depth (100).
	noLogLimit = 0

	// shortHashLen is the expected length of CommitEntry.ShortHash.
	shortHashLen = 7

	// testCommitDateStr is the fixed author date injected into all test commits
	// via GIT_AUTHOR_DATE / GIT_COMMITTER_DATE in initRepo.
	testCommitDateStr = "2024-01-01T12:00:00+00:00"
)

// testCommitDate is the time.Time equivalent of testCommitDateStr.
// TestLog_DatePopulated compares commit dates against this value.
var testCommitDate = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// initRepo creates a bare git repository in dir, sets user config, and returns
// a helper that runs git commands inside it.
func initRepo(test *testing.T, dir string) func(args ...string) {
	test.Helper()

	run := func(args ...string) {
		test.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir

		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test Author",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_DATE="+testCommitDateStr,
			"GIT_COMMITTER_DATE="+testCommitDateStr,
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			test.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test Author")

	return run
}

// addCommit writes a file and creates a commit with the given message.
func addCommit(test *testing.T, dir, message string, git func(...string)) {
	test.Helper()

	filePath := filepath.Join(dir, message+".txt")
	if err := os.WriteFile(filePath, []byte(message), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}

	git("add", ".")
	git("commit", "-m", message)
}

// mustOpen opens the repository at dir and fails the test on error.
func mustOpen(test *testing.T, dir string) *git.RepoState {
	test.Helper()
	repoState, err := git.Open(dir)
	if err != nil {
		test.Fatalf("git.Open(%q): %v", dir, err)
	}
	return repoState
}

// mustLog calls Log on repoState and fails the test on error.
func mustLog(test *testing.T, repoState *git.RepoState, limit int) []git.CommitEntry {
	test.Helper()
	entries, err := git.Log(repoState, limit)
	if err != nil {
		test.Fatalf("git.Log: %v", err)
	}
	return entries
}

// makeRemote creates a bare git repository in a temp directory and returns its path.
func makeRemote(test *testing.T) string {
	test.Helper()
	dir := test.TempDir()
	cmd := exec.Command("git", "init", "--bare", "-b", "main", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		test.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return dir
}

// TestOpen_ValidRepo verifies that a normal repository opens without error.
func TestOpen_ValidRepo(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)

	addCommit(test, dir, "initial commit", gitCmd)

	repoState := mustOpen(test, dir)

	if repoState.Branch != "main" {
		test.Errorf("Branch = %q, want %q", repoState.Branch, "main")
	}
	if filepath.Clean(repoState.Path) != filepath.Clean(dir) {
		test.Errorf("Path = %q, want %q", repoState.Path, dir)
	}
}

// TestOpen_NonRepo verifies that opening a non-repository returns an error.
func TestOpen_NonRepo(test *testing.T) {
	dir := test.TempDir()
	_, err := git.Open(dir)
	if err == nil {
		test.Fatal("expected error for non-repo directory, got nil")
	}
}

// TestOpen_DetachedHead verifies that a detached HEAD is detected and reported.
func TestOpen_DetachedHead(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)

	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	// Detach HEAD by checking out a specific commit hash.
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD~1").Output()
	if err != nil {
		test.Fatalf("rev-parse: %v", err)
	}

	hash := string(out[:len(out)-1]) // trim newline
	gitCmd("checkout", "--detach", hash)

	_, openErr := git.Open(dir)
	if !errors.Is(openErr, git.ErrDetachedHead) {
		test.Fatalf("expected ErrDetachedHead, got %v", openErr)
	}
}

// TestOpen_MergeInProgress verifies that an in-progress merge is detected.
func TestOpen_MergeInProgress(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)

	// Simulate a merge in progress by writing MERGE_HEAD.
	mergeHeadPath := filepath.Join(dir, ".git", "MERGE_HEAD")
	if err := os.WriteFile(mergeHeadPath, []byte("deadbeef\n"), 0o644); err != nil {
		test.Fatalf("WriteFile MERGE_HEAD: %v", err)
	}

	_, err := git.Open(dir)
	if !errors.Is(err, git.ErrOperationInProgress) {
		test.Fatalf("expected ErrOperationInProgress, got %v", err)
	}
}

// TestOpen_NoRemote verifies repos without a remote are handled gracefully.
func TestOpen_NoRemote(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)

	addCommit(test, dir, "commit", gitCmd)

	repoState := mustOpen(test, dir)

	if repoState.HasRemote {
		test.Error("HasRemote should be false for a repo with no remotes")
	}

	if repoState.HasUpstream {
		test.Error("HasUpstream should be false for a repo with no remotes")
	}

	if len(repoState.UnpushedHashes) == 0 {
		test.Error("all commits should be unpushed when there is no upstream")
	}
}

// TestOpen_WithUpstream verifies that unpushed commits are correctly identified
// when a remote tracking branch exists.
func TestOpen_WithUpstream(test *testing.T) {
	const wantUnpushed = 2

	remoteDir := makeRemote(test)

	// Create a local repo, push one commit, then add two more locally.
	localDir := test.TempDir()
	gitCmd := initRepo(test, localDir)
	addCommit(test, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(test, localDir, "unpushed-1", gitCmd)
	addCommit(test, localDir, "unpushed-2", gitCmd)

	repoState := mustOpen(test, localDir)

	if !repoState.HasRemote {
		test.Error("HasRemote should be true")
	}
	if !repoState.HasUpstream {
		test.Error("HasUpstream should be true")
	}
	if len(repoState.UnpushedHashes) != wantUnpushed {
		test.Errorf("UnpushedHashes len = %d, want %d", len(repoState.UnpushedHashes), wantUnpushed)
	}
}

// TestLog_ReturnsEntries verifies that Log returns the correct number of entries.
func TestLog_ReturnsEntries(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "commit-one", gitCmd)
	addCommit(test, dir, "commit-two", gitCmd)
	addCommit(test, dir, "commit-three", gitCmd)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)

	if len(entries) != 3 {
		test.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

// TestLog_RespectsDepthLimit verifies that the limit parameter is honoured.
func TestLog_RespectsDepthLimit(test *testing.T) {
	const (
		totalCommits = 5
		depthLimit   = 3
	)

	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	for i := 0; i < totalCommits; i++ {
		addCommit(test, dir, fmt.Sprintf("commit-%d", i), gitCmd)
	}

	entries := mustLog(test, mustOpen(test, dir), depthLimit)

	if len(entries) != depthLimit {
		test.Errorf("len(entries) = %d, want %d (depth limit)", len(entries), depthLimit)
	}
}

// TestLog_ShortHashLength verifies that ShortHash is 7 characters.
func TestLog_ShortHashLength(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "a commit", gitCmd)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)

	if len(entries[0].ShortHash) != shortHashLen {
		test.Errorf("ShortHash len = %d, want %d", len(entries[0].ShortHash), shortHashLen)
	}
}

// TestLog_IsUnpushedFlag verifies pushed/unpushed flags on entries.
func TestLog_IsUnpushedFlag(test *testing.T) {
	const wantEntries = 2

	remoteDir := makeRemote(test)

	localDir := test.TempDir()
	gitCmd := initRepo(test, localDir)
	addCommit(test, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(test, localDir, "unpushed", gitCmd)

	entries := mustLog(test, mustOpen(test, localDir), 0)

	if len(entries) != wantEntries {
		test.Fatalf("len(entries) = %d, want %d", len(entries), wantEntries)
	}

	if !entries[0].IsUnpushed {
		test.Error("entries[0] (unpushed) should have IsUnpushed=true")
	}

	if entries[1].IsUnpushed {
		test.Error("entries[1] (pushed) should have IsUnpushed=false")
	}
}

// TestLog_DatePopulated verifies that commit dates are non-zero.
func TestLog_DatePopulated(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "dated commit", gitCmd)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)

	if entries[0].Date.IsZero() {
		test.Error("Date should not be zero")
	}

	if !entries[0].Date.Equal(testCommitDate) {
		test.Errorf("Date = %v, want %v", entries[0].Date, testCommitDate)
	}
}

// TestOpen_RebaseInProgress verifies that an in-progress rebase is detected.
func TestOpen_RebaseInProgress(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)

	// Simulate a rebase in progress by creating the rebase-merge directory.
	rebaseMergePath := filepath.Join(dir, ".git", "rebase-merge")
	if err := os.MkdirAll(rebaseMergePath, 0o755); err != nil {
		test.Fatalf("MkdirAll rebase-merge: %v", err)
	}

	_, err := git.Open(dir)
	if !errors.Is(err, git.ErrOperationInProgress) {
		test.Fatalf("expected ErrOperationInProgress, got %v", err)
	}
}

// TestLog_MessageIsFirstLine verifies that only the subject line of a multi-line
// commit message is returned.
func TestLog_MessageIsFirstLine(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)

	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}

	gitCmd("add", ".")
	gitCmd("commit", "-m", "subject line\n\nThis is the body paragraph.")

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)

	if entries[0].Message != "subject line" {
		test.Errorf("Message = %q, want %q", entries[0].Message, "subject line")
	}
}

// TestLog_CommitOrder verifies that the log is returned newest-first.
func TestLog_CommitOrder(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first-commit", gitCmd)
	addCommit(test, dir, "second-commit", gitCmd)
	addCommit(test, dir, "third-commit", gitCmd)

	entries := mustLog(test, mustOpen(test, dir), noLogLimit)

	if len(entries) != 3 {
		test.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].Message != "third-commit" {
		test.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "third-commit")
	}

	if entries[2].Message != "first-commit" {
		test.Errorf("entries[2].Message = %q, want %q", entries[2].Message, "first-commit")
	}
}
