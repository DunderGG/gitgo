package git

import (
	"fmt"
	"os/exec"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// AmendCommit rewrites the HEAD commit with the values in opts, keeping the
// existing file tree and parent chain intact. It is equivalent to
// `git commit --amend --reset-author`.
//
// Returns ErrCommitNotUnpushed if the HEAD commit is not in
// state.UnpushedHashes — pushed commits must not be rewritten.
func AmendCommit(state *RepoState, opts AmendOptions) error {
	head, err := state.Repo.Head()
	if err != nil {
		return fmt.Errorf("reading HEAD: %w", err)
	}

	headHash := head.Hash()
	if !state.UnpushedHashes[headHash] {
		return ErrCommitNotUnpushed
	}

	headCommit, err := state.Repo.CommitObject(headHash)
	if err != nil {
		return fmt.Errorf("loading HEAD commit: %w", err)
	}

	// object.Signature is go-git's combined author/committer identity.
	// Setting Committer == Author mirrors what `git commit --amend --reset-author` does.
	sig := object.Signature{
		Name:  opts.AuthorName,
		Email: opts.AuthorEmail,
		When:  opts.Date,
	}

	// Build the replacement commit. TreeHash is the root tree object (the
	// directory snapshot) — we keep it unchanged because we're only editing
	// metadata, not file contents. ParentHashes preserves the commit's
	// position in the graph.
	newCommit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      opts.Message,
		TreeHash:     headCommit.TreeHash,
		ParentHashes: headCommit.ParentHashes,
	}

	newHash, err := storeCommit(state, newCommit)
	if err != nil {
		return fmt.Errorf("storing amended commit: %w", err)
	}

	// HEAD is a symbolic ref pointing at refs/heads/<branch>. We update the
	// branch ref directly rather than HEAD itself, which is the correct way to
	// move a branch tip in git's object model.
	newRef := plumbing.NewHashReference(head.Name(), newHash)
	if err := state.Repo.Storer.SetReference(newRef); err != nil {
		return fmt.Errorf("updating branch ref: %w", err)
	}

	return nil
}

// RebaseRewrite rewrites a single unpushed commit anywhere in history by
// rebuilding the first-parent chain from the target commit up to HEAD.
// Commits above the target are rebuilt with the same tree and metadata but
// updated parent hashes; only the target receives the values in opts.
//
// Merge commits in the chain are rebuilt with their non-first parents
// preserved unchanged.
//
// Returns ErrCommitNotUnpushed if targetHash is not in state.UnpushedHashes.
func RebaseRewrite(state *RepoState, targetHash plumbing.Hash, opts AmendOptions) error {
	if !state.UnpushedHashes[targetHash] {
		return ErrCommitNotUnpushed
	}

	head, err := state.Repo.Head()
	if err != nil {
		return fmt.Errorf("reading HEAD: %w", err)
	}
	headHash := head.Hash()

	// Collect the chain from HEAD down to targetHash, inclusive.
	// chain[0] == HEAD, chain[len-1] == target.
	chain, err := collectChain(state, headHash, targetHash)
	if err != nil {
		return err
	}

	// Walk the chain bottom-up (target first, HEAD last) so that by the time we
	// rebuild a commit we have already computed the new hash for its parent.
	// oldToNew maps each original commit hash to its replacement, letting us
	// fix up parent pointers as we go.
	oldToNew := make(map[plumbing.Hash]plumbing.Hash, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		original := chain[i]

		// Replace any parent hash that was already rebuilt so the chain stays
		// connected. Parents that are below the target (i.e. not in oldToNew)
		// keep their original hashes unchanged.
		newParents := make([]plumbing.Hash, len(original.ParentHashes))
		for j, ph := range original.ParentHashes {
			if rebuilt, ok := oldToNew[ph]; ok {
				newParents[j] = rebuilt
			} else {
				newParents[j] = ph
			}
		}

		var rebuilt *object.Commit
		if original.Hash == targetHash {
			// This is the commit the user wants to edit. Replace its author,
			// committer and message with the values from opts while keeping the
			// original tree (file snapshot) and the (possibly remapped) parents.
			// Setting Committer == Author is the same behaviour as
			// `git commit --amend --reset-author`.
			sig := object.Signature{
				Name:  opts.AuthorName,
				Email: opts.AuthorEmail,
				When:  opts.Date,
			}
			rebuilt = &object.Commit{
				Author:       sig,
				Committer:    sig,
				Message:      opts.Message,
				TreeHash:     original.TreeHash,
				ParentHashes: newParents,
			}
		} else {
			// This commit is above the target — its content is unchanged, but
			// its parent pointer may have been remapped, so we must store a new
			// object. Git hashes include parent hashes, so even an identical
			// commit with a different parent produces a different hash.
			rebuilt = &object.Commit{
				Author:       original.Author,
				Committer:    original.Committer,
				Message:      original.Message,
				TreeHash:     original.TreeHash,
				ParentHashes: newParents,
			}
		}

		// Store the rebuilt commit and record its new hash.
		newHash, storeErr := storeCommit(state, rebuilt)
		if storeErr != nil {
			return fmt.Errorf("rebuilding commit %s: %w", original.Hash, storeErr)
		}
		oldToNew[original.Hash] = newHash
	}

	// Point the branch ref at the rebuilt HEAD.
	newHeadHash := oldToNew[headHash]
	newRef := plumbing.NewHashReference(head.Name(), newHeadHash)
	if setErr := state.Repo.Storer.SetReference(newRef); setErr != nil {
		// Restore the original ref so the repo is left in a consistent state.
		origRef := plumbing.NewHashReference(head.Name(), headHash)
		_ = state.Repo.Storer.SetReference(origRef)
		return fmt.Errorf("updating branch ref: %w", setErr)
	}

	return nil
}

// storeCommit encodes commit and writes it to the object store, returning the
// resulting hash. In git's content-addressable storage, the hash is derived
// from the serialised object bytes, so two identical commits always produce
// the same hash and are deduplicated automatically.
func storeCommit(state *RepoState, commit *object.Commit) (plumbing.Hash, error) {
	// NewEncodedObject gives us an in-memory buffer that Encode writes into.
	obj := state.Repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	// SetEncodedObject persists the buffer and returns the SHA-1 hash.
	return state.Repo.Storer.SetEncodedObject(obj)
}

// collectChain walks from headHash following the first parent of each commit
// until targetHash is reached (inclusive), returning commits in HEAD-first
// order. Returns an error when targetHash is not reachable.
//
// We follow only the first parent so that the chain stays linear even when
// merge commits are present. Non-first parents (the merged-in branches) are
// preserved as-is in the rebuilt commits by the caller.
func collectChain(state *RepoState, headHash, targetHash plumbing.Hash) ([]*object.Commit, error) {
	var chain []*object.Commit
	current := headHash
	for {
		commit, err := state.Repo.CommitObject(current)
		if err != nil {
			return nil, fmt.Errorf("loading commit %s: %w", current, err)
		}
		chain = append(chain, commit)
		if current == targetHash {
			return chain, nil
		}
		// A commit with no parents is the very first commit in the repo.
		// If we reach it without finding targetHash, the target is not in
		// this branch's first-parent history.
		if len(commit.ParentHashes) == 0 {
			return nil, fmt.Errorf("commit %s is not reachable from HEAD", targetHash)
		}
		current = commit.ParentHashes[0]
	}
}

// IsDirty reports whether the working tree has any uncommitted changes to
// tracked files (staged or unstaged). Untracked-only new files are excluded
// because they are not touched by rewrite operations and cannot be stashed
// without the -u flag.
func IsDirty(state *RepoState) (bool, error) {
	workingTree, err := state.Repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("accessing worktree: %w", err)
	}

	// Status() walks the index and working tree to produce a file-by-file
	// status map, similar to `git status --porcelain`.
	status, err := workingTree.Status()
	if err != nil {
		return false, fmt.Errorf("reading worktree status: %w", err)
	}

	for _, fileStatus := range status {
		// Staging == Untracked && Worktree == Untracked means the file is
		// brand-new and not tracked by git at all — safe to ignore.
		if fileStatus.Staging == gogit.Untracked && fileStatus.Worktree == gogit.Untracked {
			continue
		}
		return true, nil
	}
	return false, nil
}

// FindGitBinary returns the absolute path to the native git binary on PATH.
// Returns ErrNativeGitNotFound when git is not available.
func FindGitBinary() (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", ErrNativeGitNotFound
	}
	return path, nil
}

// AutoStash runs `git stash` in the repository's working tree, saving any
// uncommitted tracked-file changes so that the rewrite can proceed on a clean
// tree. gitBin must be the absolute path returned by FindGitBinary.
func AutoStash(state *RepoState, gitBin string) error {
	cmd := exec.Command(gitBin, "stash")
	cmd.Dir = state.Path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git stash: %w\n%s", err, out)
	}
	return nil
}

// AutoStashPop runs `git stash pop` to restore the changes saved by
// AutoStash. It is called after the rewrite completes (whether or not the
// rewrite succeeded) so that the user's work is never left trapped in the
// stash. gitBin must be the absolute path returned by FindGitBinary.
func AutoStashPop(state *RepoState, gitBin string) error {
	cmd := exec.Command(gitBin, "stash", "pop")
	cmd.Dir = state.Path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git stash pop: %w\n%s", err, out)
	}
	return nil
}
