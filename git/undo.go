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
//
// The move is recorded in the reflog like any other GitGo branch update.
func ResetBranch(state *RepoState, branch plumbing.ReferenceName, expectedTip, targetHash plumbing.Hash) error {
	// Make sure the target commit still exists before pointing a branch at it.
	if _, err := state.Repo.CommitObject(targetHash); err != nil {
		return fmt.Errorf("loading commit %s: %w", targetHash, err)
	}

	err := moveBranch(state, branch, expectedTip, targetHash, reflogUndo)
	if errors.Is(err, ErrBranchChanged) {
		return ErrBranchMoved
	}
	return err
}
