package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// AmendCommit rewrites the tip commit of state.Branch with the values in opts,
// keeping the existing file tree and parent chain intact. The committer is
// kept (see editedSignatures).
//
// The branch does not need to be checked out; only its ref is moved.
//
// Branches in opts.MoveBranches are moved along; if any cannot be moved the
// edit still stands and a *RefsNotMovedError is returned.
//
// Returns ErrCommitNotUnpushed if the tip commit is not in
// state.UnpushedHashes — pushed commits must not be rewritten.
func AmendCommit(state *RepoState, opts AmendOptions) error {
	head, err := branchTip(state)
	if err != nil {
		return err
	}

	if err := validateIdentity(opts); err != nil {
		return err
	}

	headHash := head.Hash()
	if !state.UnpushedHashes[headHash] {
		return ErrCommitNotUnpushed
	}

	headCommit, err := state.Repo.CommitObject(headHash)
	if err != nil {
		return fmt.Errorf("loading HEAD commit: %w", err)
	}

	author, committer := editedSignatures(headCommit, opts)

	// Build the replacement commit. The tree (the directory snapshot) is kept
	// unchanged because we're only editing metadata, not file contents, and the
	// parents keep the commit's position in the graph.
	newCommit := rebuildCommit(headCommit, author, committer, opts.Message, headCommit.ParentHashes)

	newHash, err := storeCommit(state, newCommit)
	if err != nil {
		return fmt.Errorf("storing amended commit: %w", err)
	}

	// head is the branch ref (refs/heads/<branch>). When the branch is checked
	// out, HEAD is a symbolic ref to it, so moving the branch ref also moves
	// HEAD — the correct way to move a branch tip in git's object model.
	message := reflogEditMessage(headHash)
	if err := moveBranch(state, head.Name(), headHash, newHash, message); err != nil {
		return err
	}
	return moveOtherBranches(state, opts.MoveBranches, map[plumbing.Hash]plumbing.Hash{headHash: newHash}, message)
}

// RebaseRewrite rewrites a single unpushed commit anywhere in history by
// rebuilding the first-parent chain from the target commit up to the tip of
// state.Branch. The branch does not need to be checked out.
// Commits above the target are rebuilt with the same tree and metadata but
// updated parent hashes; only the target receives the values in opts.
//
// Merge commits in the chain are rebuilt with their non-first parents
// preserved unchanged.
//
// Branches in opts.MoveBranches are moved along; if any cannot be moved the
// edit still stands and a *RefsNotMovedError is returned.
//
// Returns ErrCommitNotUnpushed if targetHash is not in state.UnpushedHashes.
func RebaseRewrite(state *RepoState, targetHash plumbing.Hash, opts AmendOptions) error {
	if err := validateIdentity(opts); err != nil {
		return err
	}
	if !state.UnpushedHashes[targetHash] {
		return ErrCommitNotUnpushed
	}

	head, err := branchTip(state)
	if err != nil {
		return err
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
			// This is the commit the user wants to edit. Replace its author
			// and message with the values from opts while keeping the
			// original tree (file snapshot) and the (possibly remapped) parents.
			author, committer := editedSignatures(original, opts)
			rebuilt = rebuildCommit(original, author, committer, opts.Message, newParents)
		} else {
			// This commit is above the target — its content is unchanged, but
			// its parent pointer may have been remapped, so we must store a new
			// object. Git hashes include parent hashes, so even an identical
			// commit with a different parent produces a different hash.
			rebuilt = rebuildCommit(original, original.Author, original.Committer, original.Message, newParents)
		}

		// Store the rebuilt commit and record its new hash.
		newHash, storeErr := storeCommit(state, rebuilt)
		if storeErr != nil {
			return fmt.Errorf("rebuilding commit %s: %w", original.Hash, storeErr)
		}
		oldToNew[original.Hash] = newHash
	}

	// Point the branch ref at the rebuilt HEAD. The rebuilt commits are only
	// new objects until this succeeds, so a failure leaves the branch as it was.
	message := reflogEditMessage(targetHash)
	if err := moveBranch(state, head.Name(), headHash, oldToNew[headHash], message); err != nil {
		return err
	}
	return moveOtherBranches(state, opts.MoveBranches, oldToNew, message)
}

// reflogEditMessage is the reflog message for an edit of the given commit.
func reflogEditMessage(target plumbing.Hash) string {
	return reflogEditPrefix + target.String()[:7]
}

// editedSignatures builds the new author and committer signatures for
// original from opts.
//
// A zero opts.Date keeps the original author date, including its time zone
// offset, so edits that do not touch the date leave it byte-for-byte intact.
//
// The original committer name, email and date are always kept, except that
// opts.SyncCommitterDate replaces the committer date with the new author date.
func editedSignatures(original *object.Commit, opts AmendOptions) (author, committer object.Signature) {
	when := opts.Date
	if when.IsZero() {
		when = original.Author.When
	}
	author = object.Signature{
		Name:  opts.AuthorName,
		Email: opts.AuthorEmail,
		When:  when,
	}

	committer = original.Committer
	if opts.SyncCommitterDate {
		committer.When = when
	}
	return author, committer
}

// rebuildCommit returns a copy of original with the given identities, message
// and parents. The tree and the headers that describe the commit's content
// (encoding, an embedded merge tag, other extra headers) are kept.
//
// Signatures (gpgsig, gpgsig-sha256) are dropped: they cover the original
// bytes, so on the rebuilt commit they would no longer verify.
func rebuildCommit(original *object.Commit, author, committer object.Signature, message string, parents []plumbing.Hash) *object.Commit {
	var extraHeaders []object.ExtraHeader
	for _, header := range original.ExtraHeaders {
		if !strings.HasPrefix(header.Key, "gpgsig") {
			extraHeaders = append(extraHeaders, header)
		}
	}
	return &object.Commit{
		Author:       author,
		Committer:    committer,
		MergeTag:     original.MergeTag,
		Message:      message,
		TreeHash:     original.TreeHash,
		ParentHashes: parents,
		Encoding:     original.Encoding,
		ExtraHeaders: extraHeaders,
	}
}

// validateIdentity rejects an author name or email that would produce a
// malformed commit header, which `git fsck` and many servers refuse on push:
// an empty name, or angle brackets or line breaks in either field.
func validateIdentity(opts AmendOptions) error {
	if strings.TrimSpace(opts.AuthorName) == "" {
		return fmt.Errorf("%w: the author name is empty", ErrInvalidIdentity)
	}
	if strings.ContainsAny(opts.AuthorName, "<>\r\n") {
		return fmt.Errorf("%w: the author name contains <, > or a line break", ErrInvalidIdentity)
	}
	if strings.ContainsAny(opts.AuthorEmail, "<>\r\n") {
		return fmt.Errorf("%w: the author email contains <, > or a line break", ErrInvalidIdentity)
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
