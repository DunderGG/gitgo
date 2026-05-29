package git

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ErrCommitNotUnpushed is returned when the caller tries to rewrite a commit
// that has already been pushed to a remote.
var ErrCommitNotUnpushed = errors.New("commit has already been pushed and cannot be rewritten")

// AmendOptions holds the new metadata values for a commit being amended.
// All fields are required; partial updates are not supported.
type AmendOptions struct {
	// Message is the full commit message, including a trailing newline.
	Message     string
	AuthorName  string
	AuthorEmail string
	Date        time.Time
}

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

	sig := object.Signature{
		Name:  opts.AuthorName,
		Email: opts.AuthorEmail,
		When:  opts.Date,
	}

	newCommit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      opts.Message,
		TreeHash:     headCommit.TreeHash,
		ParentHashes: headCommit.ParentHashes,
	}

	obj := state.Repo.Storer.NewEncodedObject()
	if err := newCommit.Encode(obj); err != nil {
		return fmt.Errorf("encoding new commit: %w", err)
	}

	newHash, err := state.Repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return fmt.Errorf("storing new commit: %w", err)
	}

	// HEAD is a symbolic ref (refs/heads/<branch>); update that ref so HEAD
	// resolves to the new commit without needing to touch HEAD itself.
	newRef := plumbing.NewHashReference(head.Name(), newHash)
	if err := state.Repo.Storer.SetReference(newRef); err != nil {
		return fmt.Errorf("updating branch ref: %w", err)
	}

	return nil
}
