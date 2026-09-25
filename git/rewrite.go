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
	return RewriteCommits(state, map[plumbing.Hash]CommitEdit{head.Hash(): amendEdit(opts)}, opts.MoveBranches)
}

// RebaseRewrite rewrites a single unpushed commit anywhere in history with the
// values in opts; see RewriteCommits for how the commits above it are rebuilt.
//
// Branches in opts.MoveBranches are moved along; if any cannot be moved the
// edit still stands and a *RefsNotMovedError is returned.
//
// Returns ErrCommitNotUnpushed if targetHash is not in state.UnpushedHashes.
func RebaseRewrite(state *RepoState, targetHash plumbing.Hash, opts AmendOptions) error {
	if err := validateIdentity(opts); err != nil {
		return err
	}
	return RewriteCommits(state, map[plumbing.Hash]CommitEdit{targetHash: amendEdit(opts)}, opts.MoveBranches)
}

// ShiftDates moves the author date of each commit in hashes by opts.Shift, in
// one rewrite (see RewriteCommits). Each commit keeps its own time zone offset,
// identity and message. The committer date is shifted by the same amount when
// opts.ShiftCommitter is set, and kept otherwise.
//
// Returns ErrCommitNotUnpushed if any commit is not in state.UnpushedHashes.
func ShiftDates(state *RepoState, hashes []plumbing.Hash, opts ShiftOptions) error {
	if opts.Shift == 0 {
		return fmt.Errorf("the date shift is zero")
	}
	edits := make(map[plumbing.Hash]CommitEdit, len(hashes))
	for _, hash := range hashes {
		edits[hash] = shiftEdit(opts)
	}
	return RewriteCommits(state, edits, opts.MoveBranches)
}

// shiftEdit is the CommitEdit that moves a commit's dates by opts.Shift.
// time.Time.Add keeps the location, so the offset is unchanged.
func shiftEdit(opts ShiftOptions) CommitEdit {
	return func(original *object.Commit) (object.Signature, object.Signature, string) {
		author, committer := original.Author, original.Committer
		author.When = author.When.Add(opts.Shift)
		if opts.ShiftCommitter {
			committer.When = committer.When.Add(opts.Shift)
		}
		return author, committer, original.Message
	}
}

// CommitEdit returns the new author, committer and message for original. The
// tree, parents and other headers are handled by RewriteCommits.
type CommitEdit func(original *object.Commit) (author, committer object.Signature, message string)

// amendEdit is the CommitEdit that applies opts to a commit.
func amendEdit(opts AmendOptions) CommitEdit {
	return func(original *object.Commit) (object.Signature, object.Signature, string) {
		author, committer := editedSignatures(original, opts)
		return author, committer, opts.Message
	}
}

// RewriteCommits applies each edit to its commit in one pass, by rebuilding
// the first-parent chain from the tip of state.Branch down to the oldest
// edited commit. The branch does not need to be checked out; only its ref is
// moved, once and with a single reflog entry, so one undo reverts the whole
// rewrite.
//
// Commits in the chain without an edit are rebuilt with the same tree and
// metadata but updated parent hashes. Merge commits keep their non-first
// parents unchanged.
//
// Branches in moveBranches are moved along; if any cannot be moved the
// rewrite still stands and a *RefsNotMovedError is returned.
//
// Returns ErrCommitNotUnpushed if any edited commit is not in
// state.UnpushedHashes, and an error if one is not on the branch's
// first-parent chain. The branch is left unchanged in both cases.
func RewriteCommits(state *RepoState, edits map[plumbing.Hash]CommitEdit, moveBranches []string) error {
	if len(edits) == 0 {
		return fmt.Errorf("no commits to rewrite")
	}
	targets := make([]plumbing.Hash, 0, len(edits))
	for hash := range edits {
		if !state.UnpushedHashes[hash] {
			return ErrCommitNotUnpushed
		}
		targets = append(targets, hash)
	}

	head, err := branchTip(state)
	if err != nil {
		return err
	}
	headHash := head.Hash()

	// Collect the chain from HEAD down to the oldest target, inclusive.
	// chain[0] == HEAD.
	chain, err := collectChain(state, headHash, targets...)
	if err != nil {
		return err
	}

	// Walk the chain bottom-up (oldest target first, HEAD last) so that by the
	// time we rebuild a commit we have already computed the new hash for its
	// parent. oldToNew maps each original commit hash to its replacement,
	// letting us fix up parent pointers as we go.
	oldToNew := make(map[plumbing.Hash]plumbing.Hash, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		original := chain[i]

		// Replace any parent hash that was already rebuilt so the chain stays
		// connected. Parents below the oldest target (i.e. not in oldToNew)
		// keep their original hashes unchanged.
		newParents := make([]plumbing.Hash, len(original.ParentHashes))
		for j, ph := range original.ParentHashes {
			if rebuilt, ok := oldToNew[ph]; ok {
				newParents[j] = rebuilt
			} else {
				newParents[j] = ph
			}
		}

		// Edited commits get their new metadata; the tree (file snapshot) is
		// always kept. The other commits are unchanged apart from their parent
		// pointer, but must still be stored as new objects: git hashes include
		// parent hashes, so an identical commit with a different parent
		// produces a different hash.
		author, committer, message := original.Author, original.Committer, original.Message
		if edit, ok := edits[original.Hash]; ok {
			author, committer, message = edit(original)
		}
		rebuilt := rebuildCommit(original, author, committer, message, newParents)

		// Store the rebuilt commit and record its new hash.
		newHash, storeErr := storeCommit(state, rebuilt)
		if storeErr != nil {
			return fmt.Errorf("rebuilding commit %s: %w", original.Hash, storeErr)
		}
		oldToNew[original.Hash] = newHash
	}

	// Point the branch ref at the rebuilt HEAD. The rebuilt commits are only
	// new objects until this succeeds, so a failure leaves the branch as it was.
	// When the branch is checked out, HEAD is a symbolic ref to it, so moving
	// the branch ref also moves HEAD.
	message := reflogRewriteMessage(targets)
	if err := moveBranch(state, head.Name(), headHash, oldToNew[headHash], message); err != nil {
		return err
	}
	return moveOtherBranches(state, moveBranches, oldToNew, message)
}

// reflogRewriteMessage is the reflog message for a rewrite of the given
// commits: the short hash for a single commit, the count for several.
func reflogRewriteMessage(targets []plumbing.Hash) string {
	if len(targets) == 1 {
		return reflogEditPrefix + targets[0].String()[:7]
	}
	return fmt.Sprintf(reflogEditManyFormat, len(targets))
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
// until every target has been seen, returning the commits in HEAD-first order
// down to the oldest target (inclusive). Returns an error when a target is not
// reachable this way.
//
// We follow only the first parent so that the chain stays linear even when
// merge commits are present. Non-first parents (the merged-in branches) are
// preserved as-is in the rebuilt commits by the caller.
func collectChain(state *RepoState, headHash plumbing.Hash, targets ...plumbing.Hash) ([]*object.Commit, error) {
	remaining := make(map[plumbing.Hash]bool, len(targets))
	for _, target := range targets {
		remaining[target] = true
	}

	var chain []*object.Commit
	current := headHash
	for {
		commit, err := state.Repo.CommitObject(current)
		if err != nil {
			return nil, fmt.Errorf("loading commit %s: %w", current, err)
		}
		chain = append(chain, commit)
		delete(remaining, current)
		if len(remaining) == 0 {
			return chain, nil
		}
		// A commit with no parents is the very first commit in the repo.
		// If we reach it with targets left, they are not in this branch's
		// first-parent history.
		if len(commit.ParentHashes) == 0 {
			for missing := range remaining {
				return nil, fmt.Errorf("commit %s is not reachable from HEAD", missing)
			}
		}
		current = commit.ParentHashes[0]
	}
}
