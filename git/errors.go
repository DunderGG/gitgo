package git

import (
	"errors"
)

// ErrCommitNotUnpushed is returned when the caller tries to rewrite a commit
// that has already been pushed to a remote.
var ErrCommitNotUnpushed = errors.New("commit has already been pushed and cannot be rewritten")

// ErrNativeGitNotFound is returned when the working tree is dirty and the
// native git binary cannot be found on PATH. go-git has no stash API, so
// auto-stash requires shelling out to the system git.
var ErrNativeGitNotFound = errors.New("git binary not found on PATH; commit or clean up working tree changes before editing commits")

// ErrDetachedHead is returned when the repository is in a detached HEAD state.
var ErrDetachedHead = errors.New("repository is in detached HEAD state; attach to a branch before using GitGo")

// ErrOperationInProgress is returned when a git operation (merge, rebase, cherry-pick) is already underway.
var ErrOperationInProgress = errors.New("a git operation is already in progress; complete or abort it before using GitGo")
