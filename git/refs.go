package git

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// AffectedRefKind classifies a ref that an edit would leave behind.
type AffectedRefKind string

const (
	// AffectedBranch is a local branch pointing at a rewritten commit. It can
	// be moved to the commit's new copy (see AmendOptions.MoveBranches).
	AffectedBranch AffectedRefKind = "branch"
	// AffectedTag is a tag pointing at a rewritten commit. Tags are never
	// moved; they keep pointing at the old commit.
	AffectedTag AffectedRefKind = "tag"
	// AffectedForkedBranch is a local branch with its own commits on top of a
	// rewritten commit. It keeps the old copy of that commit in its history.
	AffectedForkedBranch AffectedRefKind = "forked-branch"
)

// AffectedRef is a ref (other than the edited branch) that points at, or
// builds on, a commit an edit would rewrite.
type AffectedRef struct {
	// Name is the short ref name, e.g. "feature/x" or "v1.0".
	Name string
	Kind AffectedRefKind
}

// RefsNotMovedError is returned by AmendCommit / RebaseRewrite when the edit
// itself succeeded but some of the requested branches could not be moved.
type RefsNotMovedError struct {
	Branches []string
	Err      error
}

func (err *RefsNotMovedError) Error() string {
	return fmt.Sprintf("commit updated, but could not move branch %s: %v", strings.Join(err.Branches, ", "), err.Err)
}

func (err *RefsNotMovedError) Unwrap() error { return err.Err }

// FindAffectedRefs returns the local branches and tags, other than
// state.Branch, that an edit of targetHash would leave pointing at old
// commits: everything that points at a commit in the rewritten chain (the
// first-parent chain from the branch tip down to targetHash), plus local
// branches whose history contains one of those commits.
//
// Results are sorted by kind, then name.
func FindAffectedRefs(state *RepoState, targetHash plumbing.Hash) ([]AffectedRef, error) {
	tip, err := branchTip(state)
	if err != nil {
		return nil, err
	}
	chain, err := collectChain(state, tip.Hash(), targetHash)
	if err != nil {
		return nil, err
	}
	rewritten := make(map[plumbing.Hash]bool, len(chain))
	for _, commit := range chain {
		rewritten[commit.Hash] = true
	}

	var affected []AffectedRef

	// Branches: either pointing at a rewritten commit, or forked from one.
	branches, err := state.Repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}
	var otherTips []*plumbing.Reference
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == state.Branch {
			return nil
		}
		if rewritten[ref.Hash()] {
			affected = append(affected, AffectedRef{Name: ref.Name().Short(), Kind: AffectedBranch})
		} else {
			otherTips = append(otherTips, ref)
		}
		return nil
	})
	branches.Close()
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	if len(otherTips) > 0 {
		// Commits below the target are shared with any fork and are never
		// rewritten, so a branch's walk can stop there.
		target := chain[len(chain)-1]
		below, err := reachableFrom(state.Repo, target.ParentHashes, nil)
		if err != nil {
			return nil, fmt.Errorf("walking history: %w", err)
		}
		for _, ref := range otherTips {
			history, err := reachableFrom(state.Repo, []plumbing.Hash{ref.Hash()}, below)
			if err != nil {
				return nil, fmt.Errorf("walking history of %s: %w", ref.Name().Short(), err)
			}
			if containsAny(history, rewritten) {
				affected = append(affected, AffectedRef{Name: ref.Name().Short(), Kind: AffectedForkedBranch})
			}
		}
	}

	// Tags pointing at a rewritten commit, directly or via an annotated tag.
	tags, err := state.Repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		commitHash := ref.Hash()
		if tag, tagErr := state.Repo.TagObject(ref.Hash()); tagErr == nil {
			commitHash = tag.Target
		} else if !errors.Is(tagErr, plumbing.ErrObjectNotFound) {
			return tagErr
		}
		if rewritten[commitHash] {
			affected = append(affected, AffectedRef{Name: ref.Name().Short(), Kind: AffectedTag})
		}
		return nil
	})
	tags.Close()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	kindOrder := map[AffectedRefKind]int{AffectedBranch: 0, AffectedForkedBranch: 1, AffectedTag: 2}
	sort.Slice(affected, func(i, j int) bool {
		if affected[i].Kind != affected[j].Kind {
			return kindOrder[affected[i].Kind] < kindOrder[affected[j].Kind]
		}
		return affected[i].Name < affected[j].Name
	})
	return affected, nil
}

// containsAny reports whether set contains any hash in wanted.
func containsAny(set, wanted map[plumbing.Hash]bool) bool {
	for hash := range wanted {
		if set[hash] {
			return true
		}
	}
	return false
}

// moveOtherBranches moves each named local branch whose tip was rewritten to
// the tip's new copy, like `git rebase --update-refs`. Branches that no longer
// point at a rewritten commit are left alone. Every branch is attempted;
// failures are returned together as a RefsNotMovedError.
func moveOtherBranches(state *RepoState, names []string, oldToNew map[plumbing.Hash]plumbing.Hash, message string) error {
	var failed []string
	var firstErr error
	for _, name := range names {
		if name == state.Branch {
			continue
		}
		refName := plumbing.NewBranchReferenceName(name)
		ref, err := state.Repo.Storer.Reference(refName)
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			continue
		}
		if err == nil {
			newHash, ok := oldToNew[ref.Hash()]
			if !ok {
				continue
			}
			err = moveBranch(state, refName, ref.Hash(), newHash, message)
		}
		if err != nil {
			failed = append(failed, name)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if len(failed) > 0 {
		return &RefsNotMovedError{Branches: failed, Err: firstErr}
	}
	return nil
}
