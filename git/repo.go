package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Open opens the git repository rooted at path for the checked-out branch,
// validates its state, and returns a fully populated RepoState.
//
// Errors are returned for:
//   - non-repository paths
//   - detached HEAD
//   - in-progress git operations (merge, rebase, cherry-pick, bisect)
func Open(path string) (*RepoState, error) {
	return OpenBranch(path, "")
}

// OpenBranch is like Open but targets the local branch with the given short
// name (e.g. "feature/x") instead of the checked-out branch. An empty branch
// name selects the checked-out branch. The branch does not need to be checked
// out; the working tree is never touched.
//
// The same validation as Open applies (detached HEAD and in-progress
// operations are rejected), and ErrBranchNotFound is returned when the branch
// does not exist.
func OpenBranch(path string, branch string) (*RepoState, error) {
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
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		// HEAD points at a branch that has no commits yet (fresh `git init`).
		return nil, ErrNoCommits
	}
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
	checkedOutBranch := head.Name().Short()

	branchName := branch
	if branchName == "" {
		branchName = checkedOutBranch
	}

	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrBranchNotFound, branchName)
		}
		return nil, fmt.Errorf("reading branch %s: %w", branchName, err)
	}

	// Determine remote / upstream information.
	hasRemote, hasUpstream, upstreamHash, err := resolveUpstream(repo, branchName)
	if err != nil {
		return nil, fmt.Errorf("resolving upstream: %w", err)
	}

	// Build the unpushed set.
	unpushed, err := computeUnpushed(repo, branchRef.Hash(), upstreamHash)
	if err != nil {
		return nil, fmt.Errorf("computing unpushed commits: %w", err)
	}

	return &RepoState{
		Repo:           repo,
		Path:           rootPath,
		Branch:         branchName,
		IsCheckedOut:   branchName == checkedOutBranch,
		HasRemote:      hasRemote,
		HasUpstream:    hasUpstream,
		UnpushedHashes: unpushed,
	}, nil
}

// ListBranches returns the short names of all local branches, sorted
// alphabetically.
func ListBranches(state *RepoState) ([]string, error) {
	iter, err := state.Repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}
	defer iter.Close()

	var names []string
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	sort.Strings(names)
	return names, nil
}

// branchTip returns the current reference for the branch the state targets.
// Rewrite and log code use this instead of HEAD so they work on branches that
// are not checked out.
func branchTip(state *RepoState) (*plumbing.Reference, error) {
	ref, err := state.Repo.Reference(plumbing.NewBranchReferenceName(state.Branch), true)
	if err != nil {
		return nil, fmt.Errorf("reading branch %s: %w", state.Branch, err)
	}
	return ref, nil
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

// computeUnpushed returns the commits reachable from tipHash that are NOT
// reachable from upstreamHash or from any remote-tracking ref
// (refs/remotes/*) — the equivalent of `git rev-list <tip> ^@{u} --not --remotes`.
//
// Excluding every remote-tracking ref, not just the upstream, keeps commits
// that were pushed on another branch read-only even when this branch has no
// upstream. When there are no remote-tracking refs at all, every commit
// reachable from tipHash is considered unpushed.
//
// The remote side is walked completely rather than cut short with a
// commit-date heuristic (as git does): clock skew or identical timestamps
// could otherwise mark a pushed commit as unpushed, which is the unsafe
// direction.
func computeUnpushed(repo *gogit.Repository, tipHash plumbing.Hash, upstreamHash plumbing.Hash) (map[plumbing.Hash]bool, error) {
	excludeTips, err := remoteTrackingTips(repo)
	if err != nil {
		return nil, err
	}
	if !upstreamHash.IsZero() {
		excludeTips = append(excludeTips, upstreamHash)
	}

	pushed, err := reachableFrom(repo, excludeTips, nil)
	if err != nil {
		return nil, fmt.Errorf("walking remote history: %w", err)
	}

	// Walk from the tip, stopping at pushed commits: everything below a pushed
	// commit is pushed too.
	unpushed, err := reachableFrom(repo, []plumbing.Hash{tipHash}, pushed)
	if err != nil {
		return nil, fmt.Errorf("walking branch history: %w", err)
	}
	return unpushed, nil
}

// remoteTrackingTips returns the commit hashes that refs/remotes/* point at.
// Symbolic refs such as refs/remotes/origin/HEAD are skipped because they
// resolve to a branch that is already listed.
func remoteTrackingTips(repo *gogit.Repository) ([]plumbing.Hash, error) {
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("listing references: %w", err)
	}
	defer refs.Close()

	var tips []plumbing.Hash
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() && ref.Type() == plumbing.HashReference {
			tips = append(tips, ref.Hash())
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing references: %w", err)
	}
	return tips, nil
}

// reachableFrom returns every commit reachable from starts, following all
// parents. Commits in stop (and therefore their ancestors) are not visited.
// Start hashes that do not name a commit (e.g. a remote ref pointing at a
// tag or a missing object in a shallow clone) are skipped.
func reachableFrom(repo *gogit.Repository, starts []plumbing.Hash, stop map[plumbing.Hash]bool) (map[plumbing.Hash]bool, error) {
	seen := make(map[plumbing.Hash]bool)
	pending := make([]plumbing.Hash, 0, len(starts))
	pending = append(pending, starts...)

	for len(pending) > 0 {
		hash := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[hash] || stop[hash] {
			continue
		}

		commit, err := repo.CommitObject(hash)
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			// Shallow clones have no objects below the shallow boundary.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("loading commit %s: %w", hash, err)
		}

		seen[hash] = true
		pending = append(pending, commit.ParentHashes...)
	}
	return seen, nil
}

// fileExists returns true when path exists (file or directory).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}
