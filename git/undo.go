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
// The update is a compare-and-swap: if the branch no longer points at
// expectedTip (a new commit was made, or the branch was deleted), nothing is
// changed and ErrBranchMoved is returned.
func ResetBranch(state *RepoState, branch plumbing.ReferenceName, expectedTip, targetHash plumbing.Hash) error {
	current, err := state.Repo.Reference(branch, true)
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

	// CheckAndSetReference only writes the new ref when the stored ref still
	// matches the old one, guarding against a concurrent change on disk.
	newRef := plumbing.NewHashReference(branch, targetHash)
	oldRef := plumbing.NewHashReference(branch, expectedTip)
	if err := state.Repo.Storer.CheckAndSetReference(newRef, oldRef); err != nil {
		return fmt.Errorf("updating branch ref: %w", err)
	}

	return nil
}
