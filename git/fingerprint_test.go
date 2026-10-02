package git_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitgo/git"
)

// TestHasChanged_FalseRightAfterOpen verifies that a freshly opened state
// matches the repository, also after reading it again.
func TestHasChanged_FalseRightAfterOpen(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)

	state := mustOpen(test, dir)
	if git.HasChanged(state) {
		test.Fatal("HasChanged = true right after Open")
	}
	if git.HasChanged(state) {
		test.Fatal("HasChanged = true on the second call")
	}
}

// TestHasChanged_DetectsOutsideChanges verifies each kind of change made with
// the git command line is reported, and that reopening clears it.
func TestHasChanged_DetectsOutsideChanges(test *testing.T) {
	cases := []struct {
		name   string
		change func(test *testing.T, dir string, gitCmd func(...string))
	}{
		{"new commit", func(test *testing.T, dir string, gitCmd func(...string)) {
			addCommit(test, dir, "second", gitCmd)
		}},
		{"amend", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("commit", "--amend", "-m", "amended")
		}},
		{"checkout of another branch at the same commit", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("switch", "-c", "other")
		}},
		{"branch deleted", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("switch", "-c", "other")
			gitCmd("branch", "-D", "main")
		}},
		{"refs packed and branch moved", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("pack-refs", "--all")
			gitCmd("update-ref", "refs/heads/main", "HEAD~1")
		}},
		{"remote added", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("remote", "add", "origin", makeRemote(test))
		}},
		{"push", func(test *testing.T, dir string, gitCmd func(...string)) {
			gitCmd("remote", "add", "origin", makeRemote(test))
			gitCmd("push", "origin", "main")
		}},
		{"merge in progress", func(test *testing.T, dir string, gitCmd func(...string)) {
			head := revParse(test, dir, "HEAD")
			if err := os.WriteFile(filepath.Join(dir, ".git", "MERGE_HEAD"), []byte(head.String()+"\n"), 0o644); err != nil {
				test.Fatalf("WriteFile: %v", err)
			}
		}},
	}

	for _, tc := range cases {
		test.Run(tc.name, func(test *testing.T) {
			dir := test.TempDir()
			gitCmd := initRepo(test, dir)
			addCommit(test, dir, "first", gitCmd)
			addCommit(test, dir, "second-base", gitCmd)
			state := mustOpen(test, dir)

			tc.change(test, dir, gitCmd)

			if !git.HasChanged(state) {
				test.Fatal("HasChanged = false after the change")
			}
		})
	}
}

// TestHasChanged_UpstreamAndFetch verifies that setting an upstream and a
// fetch that brings nothing new (only FETCH_HEAD is rewritten) are reported.
func TestHasChanged_UpstreamAndFetch(test *testing.T) {
	remoteDir := makeRemote(test)
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "origin", "main")

	state := mustOpen(test, dir)
	gitCmd("branch", "--set-upstream-to", "origin/main")
	if !git.HasChanged(state) {
		test.Fatal("HasChanged = false after setting the upstream")
	}

	state = mustOpen(test, dir)
	gitCmd("fetch", "origin")
	fetchHead := filepath.Join(dir, ".git", "FETCH_HEAD")
	// Make sure the time differs even on file systems with coarse timestamps.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(fetchHead, later, later); err != nil {
		test.Fatalf("Chtimes: %v", err)
	}
	if !git.HasChanged(state) {
		test.Fatal("HasChanged = false after a fetch")
	}

	if git.HasChanged(mustOpen(test, dir)) {
		test.Fatal("HasChanged = true after reopening")
	}
}

// TestHasChanged_IgnoresOtherBranchesAndWorkingTree verifies that changes the
// view does not depend on are not reported.
func TestHasChanged_IgnoresOtherBranchesAndWorkingTree(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	state := mustOpen(test, dir)

	gitCmd("branch", "other")
	gitCmd("tag", "v1")
	if err := os.WriteFile(filepath.Join(dir, "first.txt"), []byte("edited"), 0o644); err != nil {
		test.Fatalf("WriteFile: %v", err)
	}

	if git.HasChanged(state) {
		test.Fatal("HasChanged = true for a new branch, a tag and a working tree edit")
	}
}
