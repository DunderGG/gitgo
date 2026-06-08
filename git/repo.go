package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Open opens the git repository rooted at path, validates its state, and
// returns a fully populated RepoState.
//
// Errors are returned for:
//   - non-repository paths
//   - detached HEAD
//   - in-progress git operations (merge, rebase, cherry-pick, bisect)
func Open(path string) (*RepoState, error) {
	repo, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	// Resolve the real working tree root (PlainOpenWithOptions may have walked up).
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("cannot access worktree: %w", err)
	}
	rootPath := worktree.Filesystem.Root()

	// Detect in-progress operations before doing anything else.
	if err := detectInProgressOperation(rootPath); err != nil {
		return nil, err
	}

	// Resolve HEAD.
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("cannot read HEAD: %w", err)
	}
	if head.Type() != plumbing.HashReference {
		// HEAD is a symbolic ref but resolves to a hash; this should not
		// happen in practice — guard anyway.
		return nil, fmt.Errorf("unexpected HEAD reference type: %s", head.Type())
	}

	// Detect detached HEAD: Head().Name() is not a branch ref.
	if !head.Name().IsBranch() {
		return nil, ErrDetachedHead
	}
	branchName := head.Name().Short()

	// Determine remote / upstream information.
	hasRemote, hasUpstream, upstreamHash, err := resolveUpstream(repo, branchName)
	if err != nil {
		return nil, fmt.Errorf("resolving upstream: %w", err)
	}

	// Build the unpushed set.
	unpushed, err := computeUnpushed(repo, head.Hash(), upstreamHash)
	if err != nil {
		return nil, fmt.Errorf("computing unpushed commits: %w", err)
	}

	return &RepoState{
		Repo:           repo,
		Path:           rootPath,
		Branch:         branchName,
		HasRemote:      hasRemote,
		HasUpstream:    hasUpstream,
		UnpushedHashes: unpushed,
	}, nil
}

// detectInProgressOperation returns ErrOperationInProgress when any of the
// sentinel files that git writes during in-progress operations are present.
func detectInProgressOperation(rootPath string) error {
	gitDir := filepath.Join(rootPath, ".git")

	sentinels := []string{
		"MERGE_HEAD",
		"CHERRY_PICK_HEAD",
		"REVERT_HEAD",
		"BISECT_LOG",
	}
	for _, sentinel := range sentinels {
		if fileExists(filepath.Join(gitDir, sentinel)) {
			return ErrOperationInProgress
		}
	}

	// Rebase can be represented by either of these directories.
	rebaseDirs := []string{"rebase-merge", "rebase-apply"}
	for _, dir := range rebaseDirs {
		if fileExists(filepath.Join(gitDir, dir)) {
			return ErrOperationInProgress
		}
	}

	return nil
}

// resolveUpstream returns remote/upstream presence flags and the hash at the
// tip of the remote tracking branch for the given local branch.
// upstreamHash is the zero value when there is no upstream.
func resolveUpstream(repo *gogit.Repository, branchName string) (hasRemote bool, hasUpstream bool, upstreamHash plumbing.Hash, err error) {
	remotes, remoteErr := repo.Remotes()
	if remoteErr != nil {
		return false, false, plumbing.ZeroHash, fmt.Errorf("listing remotes: %w", remoteErr)
	}
	hasRemote = len(remotes) > 0
	if !hasRemote {
		return false, false, plumbing.ZeroHash, nil
	}

	// Read the branch config to find the tracking remote and merge ref.
	cfg, cfgErr := repo.Config()
	if cfgErr != nil {
		return hasRemote, false, plumbing.ZeroHash, fmt.Errorf("reading config: %w", cfgErr)
	}

	branchCfg, ok := cfg.Branches[branchName]
	if !ok || branchCfg.Remote == "" || branchCfg.Merge == "" {
		return hasRemote, false, plumbing.ZeroHash, nil
	}

	// Build the remote-tracking ref name, e.g. refs/remotes/origin/main.
	trackingRefName := plumbing.NewRemoteReferenceName(branchCfg.Remote, branchCfg.Merge.Short())
	trackingRef, refErr := repo.Reference(trackingRefName, true)
	if refErr != nil {
		// Tracking ref is configured but not fetched yet — treat as no upstream.
		return hasRemote, false, plumbing.ZeroHash, nil
	}

	return hasRemote, true, trackingRef.Hash(), nil
}

// computeUnpushed walks commits reachable from headHash and returns those that
// are NOT reachable from upstreamHash (i.e. they are strictly above the
// upstream tip). When upstreamHash is the zero value (no upstream), all
// commits in the log up to the default depth are considered unpushed.
func computeUnpushed(repo *gogit.Repository, headHash plumbing.Hash, upstreamHash plumbing.Hash) (map[plumbing.Hash]bool, error) {
	unpushed := make(map[plumbing.Hash]bool)

	// Walk the log from HEAD. We stop as soon as we encounter the upstream tip
	// because every commit reachable from there is already on the remote.
	logIter, err := repo.Log(&gogit.LogOptions{From: headHash})
	if err != nil {
		return nil, fmt.Errorf("opening log: %w", err)
	}
	defer logIter.Close()

	for {
		commit, iterErr := logIter.Next()
		if iterErr != nil {
			break // io.EOF is the normal exit; any other error also terminates
		}
		if !upstreamHash.IsZero() && commit.Hash == upstreamHash {
			// This commit is the upstream tip — it and everything below it
			// have already been pushed, so we stop here.
			break
		}
		// upstreamHash.IsZero() means there is no remote tracking branch, so
		// every commit is treated as unpushed (safe to edit).
		unpushed[commit.Hash] = true
	}

	return unpushed, nil
}

// fileExists returns true when path exists (file or directory).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}
