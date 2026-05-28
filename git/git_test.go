package git_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gitgo/git"
)

// initRepo creates a bare git repository in dir, sets user config, and returns
// a helper that runs git commands inside it.
func initRepo(t *testing.T, dir string) func(args ...string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test Author",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_DATE=2024-01-01T12:00:00+00:00",
			"GIT_COMMITTER_DATE=2024-01-01T12:00:00+00:00",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test Author")
	return run
}

// addCommit writes a file and creates a commit with the given message.
func addCommit(t *testing.T, dir, message string, git func(...string)) {
	t.Helper()
	filePath := filepath.Join(dir, message+".txt")
	if err := os.WriteFile(filePath, []byte(message), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	git("add", ".")
	git("commit", "-m", message)
}

// TestOpen_ValidRepo verifies that a normal repository opens without error.
func TestOpen_ValidRepo(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "initial commit", gitCmd)

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if state.Branch != "main" {
		t.Errorf("Branch = %q, want %q", state.Branch, "main")
	}
	if state.Path != filepath.ToSlash(dir) && state.Path != dir {
		// Accept either slash style on Windows.
		if filepath.Clean(state.Path) != filepath.Clean(dir) {
			t.Errorf("Path = %q, want %q", state.Path, dir)
		}
	}
}

// TestOpen_NonRepo verifies that opening a non-repository returns an error.
func TestOpen_NonRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := git.Open(dir)
	if err == nil {
		t.Fatal("expected error for non-repo directory, got nil")
	}
}

// TestOpen_DetachedHead verifies that a detached HEAD is detected and reported.
func TestOpen_DetachedHead(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "first", gitCmd)
	addCommit(t, dir, "second", gitCmd)

	// Detach HEAD by checking out a specific commit hash.
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD~1").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	hash := string(out[:len(out)-1]) // trim newline
	gitCmd("checkout", "--detach", hash)

	_, openErr := git.Open(dir)
	if openErr == nil {
		t.Fatal("expected ErrDetachedHead, got nil")
	}
}

// TestOpen_MergeInProgress verifies that an in-progress merge is detected.
func TestOpen_MergeInProgress(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "base", gitCmd)

	// Simulate a merge in progress by writing MERGE_HEAD.
	mergeHeadPath := filepath.Join(dir, ".git", "MERGE_HEAD")
	if err := os.WriteFile(mergeHeadPath, []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatalf("WriteFile MERGE_HEAD: %v", err)
	}

	_, err := git.Open(dir)
	if err == nil {
		t.Fatal("expected ErrOperationInProgress, got nil")
	}
}

// TestOpen_NoRemote verifies repos without a remote are handled gracefully.
func TestOpen_NoRemote(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "commit", gitCmd)

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if state.HasRemote {
		t.Error("HasRemote should be false for a repo with no remotes")
	}
	if state.HasUpstream {
		t.Error("HasUpstream should be false for a repo with no remotes")
	}
	if len(state.UnpushedHashes) == 0 {
		t.Error("all commits should be unpushed when there is no upstream")
	}
}

// TestOpen_WithUpstream verifies that unpushed commits are correctly identified
// when a remote tracking branch exists.
func TestOpen_WithUpstream(t *testing.T) {
	// Create a "remote" bare repo.
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", "-b", "main", remoteDir).Run()

	// Create a local repo, push one commit, then add two more locally.
	localDir := t.TempDir()
	gitCmd := initRepo(t, localDir)
	addCommit(t, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(t, localDir, "unpushed-1", gitCmd)
	addCommit(t, localDir, "unpushed-2", gitCmd)

	state, err := git.Open(localDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !state.HasRemote {
		t.Error("HasRemote should be true")
	}
	if !state.HasUpstream {
		t.Error("HasUpstream should be true")
	}
	if len(state.UnpushedHashes) != 2 {
		t.Errorf("UnpushedHashes len = %d, want 2", len(state.UnpushedHashes))
	}
}

// TestLog_ReturnsEntries verifies that Log returns the correct number of entries.
func TestLog_ReturnsEntries(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "commit-one", gitCmd)
	addCommit(t, dir, "commit-two", gitCmd)
	addCommit(t, dir, "commit-three", gitCmd)

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	entries, err := git.Log(state, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

// TestLog_RespectsDepthLimit verifies that the limit parameter is honoured.
func TestLog_RespectsDepthLimit(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	for i := 0; i < 5; i++ {
		addCommit(t, dir, fmt.Sprintf("commit-%d", i), gitCmd)
	}

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	entries, err := git.Log(state, 3)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3 (depth limit)", len(entries))
	}
}

// TestLog_ShortHashLength verifies that ShortHash is 7 characters.
func TestLog_ShortHashLength(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "a commit", gitCmd)

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries, err := git.Log(state, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries[0].ShortHash) != 7 {
		t.Errorf("ShortHash len = %d, want 7", len(entries[0].ShortHash))
	}
}

// TestLog_IsUnpushedFlag verifies pushed/unpushed flags on entries.
func TestLog_IsUnpushedFlag(t *testing.T) {
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", "-b", "main", remoteDir).Run()

	localDir := t.TempDir()
	gitCmd := initRepo(t, localDir)
	addCommit(t, localDir, "pushed", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	addCommit(t, localDir, "unpushed", gitCmd)

	state, err := git.Open(localDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries, err := git.Log(state, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if !entries[0].IsUnpushed {
		t.Error("entries[0] (unpushed) should have IsUnpushed=true")
	}
	if entries[1].IsUnpushed {
		t.Error("entries[1] (pushed) should have IsUnpushed=false")
	}
}

// TestLog_DatePopulated verifies that commit dates are non-zero.
func TestLog_DatePopulated(t *testing.T) {
	dir := t.TempDir()
	gitCmd := initRepo(t, dir)
	addCommit(t, dir, "dated commit", gitCmd)

	state, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries, err := git.Log(state, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if entries[0].Date.IsZero() {
		t.Error("Date should not be zero")
	}
	expected := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !entries[0].Date.Equal(expected) {
		t.Errorf("Date = %v, want %v", entries[0].Date, expected)
	}
}
