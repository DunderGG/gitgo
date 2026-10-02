package git

import (
	"errors"
)

// ErrCommitNotUnpushed is returned when the caller tries to rewrite a commit
// that has already been pushed to a remote.
var ErrCommitNotUnpushed = errors.New("commit has already been pushed and cannot be rewritten")

// ErrInvalidIdentity is returned when an author or committer name or email
// would produce a malformed commit. The wrapping error says which field is
// wrong.
var ErrInvalidIdentity = errors.New("invalid identity")

// ErrDetachedHead is returned when the repository is in a detached HEAD state.
var ErrDetachedHead = errors.New("repository is in detached HEAD state; attach to a branch before using GitGo")

// ErrOperationInProgress is returned when a git operation (merge, rebase, cherry-pick) is already underway.
var ErrOperationInProgress = errors.New("a git operation is already in progress; complete or abort it before using GitGo")

// ErrBranchMoved is returned by ResetBranch when the branch tip no longer
// matches the expected hash, e.g. because a new commit was made after the
// rewrite that is being undone.
var ErrBranchMoved = errors.New("the branch has changed since the last rewrite; undo is no longer available")

// ErrUndoPushed is returned by ResetBranch when commits created by the rewrite
// have been pushed since, so undoing it would rewrite published history.
var ErrUndoPushed = errors.New("the edited commits have been pushed since; undo is no longer available")

// ErrBranchChanged is returned when a branch no longer points where it did
// when the operation started, e.g. a commit was made from a terminal while a
// rewrite was running. The branch is left untouched.
var ErrBranchChanged = errors.New("the branch changed on disk during the operation; reload and try again")

// ErrBackupNotFound is returned when no backup has the requested ID, e.g.
// because it was deleted from a terminal.
var ErrBackupNotFound = errors.New("backup not found")

// ErrInvalidBackupName is returned for a backup name that is too long or
// spans several lines. The wrapping error says which.
var ErrInvalidBackupName = errors.New("invalid backup name")

// ErrRestorePushed is returned when restoring a backup would remove commits
// from a branch that have been pushed since the backup was made. The wrapping
// error names the branch.
var ErrRestorePushed = errors.New("restoring this backup would remove commits that have been pushed")

// ErrRestoreFilesChanged is returned when the checked-out branch's files have
// changed since the backup: moving the branch back would leave those changes
// behind as uncommitted changes. The wrapping error names the branch.
var ErrRestoreFilesChanged = errors.New("the checked-out branch has commits with file changes made since this backup; restore it from a terminal (git reset --hard) if you want to discard them")

// ErrBranchNotFound is returned by OpenBranch when the requested local branch
// does not exist.
var ErrBranchNotFound = errors.New("branch not found")

// ErrNoCommits is returned when the repository has no commits yet (e.g. right
// after `git init`), so there is no branch history to show.
var ErrNoCommits = errors.New("this repository has no commits yet; make a first commit before using GitGo")
