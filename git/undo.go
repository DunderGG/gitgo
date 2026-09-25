package git

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
)

// ResetBranch moves branch from expectedTip back to targetHash. It is used to
// undo a rewrite: rewrites only change commit metadata, never file trees, so
// moving the ref leaves the index and working tree consistent without touching
// them, whether or not the branch is checked out.
//
// state must be opened for branch (see OpenBranch) so its UnpushedHashes are
// current. The undo is refused with ErrUndoPushed when any commit it would
// discard (reachable from expectedTip but not from targetHash) has been pushed.
//
// The update is a compare-and-swap: if the branch no longer points at
// expectedTip (a new commit was made, or the branch was deleted), nothing is
// changed and ErrBranchMoved is returned.
//
// The move is recorded in the reflog like any other GitGo branch update.
func ResetBranch(state *RepoState, branch plumbing.ReferenceName, expectedTip, targetHash plumbing.Hash) error {
	if branch.Short() != state.Branch {
		return fmt.Errorf("state is for branch %s, not %s", state.Branch, branch.Short())
	}

	// Check the branch first: the pushed check below is only meaningful while
	// the branch still points at expectedTip.
	current, err := state.Repo.Storer.Reference(branch)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return ErrBranchMoved
	}
	if err != nil {
		return fmt.Errorf("reading branch %s: %w", branch.Short(), err)
	}
	if current.Hash() != expectedTip {
		return ErrBranchMoved
	}

	// Make sure the target commit still exists before pointing a branch at it.
	if _, err := state.Repo.CommitObject(targetHash); err != nil {
		return fmt.Errorf("loading commit %s: %w", targetHash, err)
	}

	kept, err := reachableFrom(state.Repo, []plumbing.Hash{targetHash}, nil)
	if err != nil {
		return fmt.Errorf("walking history: %w", err)
	}
	discarded, err := reachableFrom(state.Repo, []plumbing.Hash{expectedTip}, kept)
	if err != nil {
		return fmt.Errorf("walking history: %w", err)
	}
	for hash := range discarded {
		if !state.UnpushedHashes[hash] {
			return ErrUndoPushed
		}
	}

	err = moveBranch(state, branch, expectedTip, targetHash, reflogUndo)
	if errors.Is(err, ErrBranchChanged) {
		return ErrBranchMoved
	}
	return err
}
