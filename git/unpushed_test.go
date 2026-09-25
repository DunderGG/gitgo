package git_test

import (
	"sort"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// setupPushedRepo creates a local repo with an "origin" remote, commits the
// given messages, and pushes them with upstream tracking on main.
func setupPushedRepo(test *testing.T, pushedMessages ...string) (string, func(...string)) {
	test.Helper()
	remoteDir := makeRemote(test)
	localDir := test.TempDir()
	gitCmd := initRepo(test, localDir)
	for _, message := range pushedMessages {
		addCommit(test, localDir, message, gitCmd)
	}
	gitCmd("remote", "add", "origin", remoteDir)
	gitCmd("push", "-u", "origin", "main")
	return localDir, gitCmd
}

// revParse resolves rev to a commit hash in the repository at dir.
func revParse(test *testing.T, dir, rev string) plumbing.Hash {
	test.Helper()
	return plumbing.NewHash(gitOutputFromDir(test, dir, "git", "rev-parse", rev))
}

// assertUnpushed checks that repoState.UnpushedHashes is exactly want.
func assertUnpushed(test *testing.T, repoState *git.RepoState, want ...plumbing.Hash) {
	test.Helper()
	got := make([]string, 0, len(repoState.UnpushedHashes))
	for hash, unpushed := range repoState.UnpushedHashes {
		if unpushed {
			got = append(got, hash.String())
		}
	}
	wantStrs := make([]string, 0, len(want))
	for _, hash := range want {
		wantStrs = append(wantStrs, hash.String())
	}
	sort.Strings(got)
	sort.Strings(wantStrs)

	if len(got) != len(wantStrs) {
		test.Fatalf("UnpushedHashes = %v, want %v", got, wantStrs)
	}
	for i := range got {
		if got[i] != wantStrs[i] {
			test.Fatalf("UnpushedHashes = %v, want %v", got, wantStrs)
		}
	}
}

// TestUnpushed_DivergedBranch verifies that when the local branch and its
// upstream each have commits the other lacks, only the local-only commit is
// unpushed. Previously the upstream tip was never found by the log walk and
// every commit — including pushed ones — was marked unpushed.
func TestUnpushed_DivergedBranch(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first", "second")

	// Move the remote ahead: push a commit, then drop it locally.
	addCommit(test, localDir, "remote-only", gitCmd)
	gitCmd("push")
	gitCmd("reset", "--hard", "HEAD~1")

	addCommit(test, localDir, "local-only", gitCmd)
	localOnly := revParse(test, localDir, "HEAD")

	assertUnpushed(test, mustOpen(test, localDir), localOnly)
}

// TestUnpushed_BehindUpstream verifies that a branch strictly behind its
// upstream has no unpushed commits.
func TestUnpushed_BehindUpstream(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first", "second", "third")
	gitCmd("reset", "--hard", "HEAD~1")

	assertUnpushed(test, mustOpen(test, localDir))
}

// TestUnpushed_MergeFromPull verifies that after merging the upstream into a
// diverged branch (what `git pull` does), only the local commit and the merge
// commit are unpushed — not the older pushed history below the merge base.
func TestUnpushed_MergeFromPull(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first", "second")

	addCommit(test, localDir, "remote-only", gitCmd)
	gitCmd("push")
	gitCmd("reset", "--hard", "HEAD~1")

	addCommit(test, localDir, "local-only", gitCmd)
	localOnly := revParse(test, localDir, "HEAD")

	gitCmd("merge", "--no-edit", "origin/main")
	merge := revParse(test, localDir, "HEAD")

	assertUnpushed(test, mustOpen(test, localDir), localOnly, merge)
}

// TestUnpushed_NoUpstreamExcludesRemoteRefs verifies that on a branch without
// an upstream, commits already pushed on another remote branch are not
// considered unpushed.
func TestUnpushed_NoUpstreamExcludesRemoteRefs(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first", "second")

	gitCmd("switch", "-c", "feature")
	addCommit(test, localDir, "feature-only", gitCmd)
	featureOnly := revParse(test, localDir, "HEAD")

	repoState := mustOpen(test, localDir)
	if repoState.HasUpstream {
		test.Error("HasUpstream should be false for a branch without tracking config")
	}
	assertUnpushed(test, repoState, featureOnly)
}

// TestUnpushed_UpToDate verifies that a branch equal to its upstream has no
// unpushed commits.
func TestUnpushed_UpToDate(test *testing.T) {
	localDir, _ := setupPushedRepo(test, "first", "second")

	assertUnpushed(test, mustOpen(test, localDir))
}
