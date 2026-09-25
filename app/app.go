package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// New creates a new App instance.
func New() *App {
	return &App{}
}

// Startup is called when the Wails application starts and stores the context
// for later use by bound methods.
func (app *App) Startup(ctx context.Context) {
	app.ctx = ctx
}

// SelectDirectory opens a native directory picker dialog and returns the
// selected path. Returns an empty string if the user cancels.
func (app *App) SelectDirectory() (string, error) {
	path, err := runtime.OpenDirectoryDialog(app.ctx, runtime.OpenDialogOptions{
		Title: "Select a Git Repository",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// OpenRepository opens the git repository at the given path, validates its
// state, and returns high-level repository information. The opened state is
// retained for subsequent GetCommitLog calls.
func (app *App) OpenRepository(path string) (RepoInfo, error) {
	state, err := gitpkg.Open(path)
	if err != nil {
		return RepoInfo{}, err
	}

	app.mutex.Lock()
	app.repoState = state
	app.lastRewrite = nil
	app.mutex.Unlock()

	return RepoInfo{
		Path:        state.Path,
		Branch:      state.Branch,
		HasRemote:   state.HasRemote,
		HasUpstream: state.HasUpstream,
	}, nil
}

// GetCommitLog returns the commit history for the currently open repository.
// OpenRepository must be called before this method.
func (app *App) GetCommitLog() ([]CommitSummary, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	entries, err := gitpkg.Log(state, 0)
	if err != nil {
		return nil, err
	}

	return commitSummariesFromEntries(entries), nil
}

// commitSummariesFromEntries maps a slice of git.CommitEntry to the
// JSON-serialisable CommitSummary DTOs used by the frontend.
func commitSummariesFromEntries(entries []gitpkg.CommitEntry) []CommitSummary {
	summaries := make([]CommitSummary, len(entries))
	for index, entry := range entries {
		summaries[index] = CommitSummary{
			Hash:       entry.Hash.String(),
			ShortHash:  entry.ShortHash,
			Message:    entry.Message,
			Author:     entry.AuthorName,
			Date:       entry.Date.Format("2006-01-02T15:04:05Z07:00"),
			IsUnpushed: entry.IsUnpushed,
		}
	}
	return summaries
}

// GetCommitDetail returns full metadata for a single commit identified by its
// 40-character hex hash. This is used to populate the edit panel.
func (app *App) GetCommitDetail(hash string) (CommitDetail, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return CommitDetail{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	commitHash := plumbing.NewHash(hash)
	commit, err := state.Repo.CommitObject(commitHash)
	if err != nil {
		return CommitDetail{}, fmt.Errorf("commit not found: %w", err)
	}

	return CommitDetail{
		Hash:        commit.Hash.String(),
		Message:     commit.Message,
		AuthorName:  commit.Author.Name,
		AuthorEmail: commit.Author.Email,
		Date:        commit.Author.When.Format("2006-01-02T15:04:05Z07:00"),
		IsUnpushed:  state.UnpushedHashes[commitHash],
	}, nil
}

// RefreshLog re-opens the current repository to pick up any changes (e.g.
// after a commit rewrite) and returns an updated commit list.
// OpenRepository must be called before this method.
func (app *App) RefreshLog() ([]CommitSummary, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	newState, err := gitpkg.Open(state.Path)
	if err != nil {
		return nil, err
	}

	app.mutex.Lock()
	app.repoState = newState
	app.mutex.Unlock()

	entries, err := gitpkg.Log(newState, 0)
	if err != nil {
		return nil, err
	}

	return commitSummariesFromEntries(entries), nil
}

// UpdateCommit applies the metadata changes in req to the identified unpushed
// commit. If the working tree is dirty, changes are automatically stashed
// before the rewrite and restored afterwards.
//
// Under the hood, HEAD rewrites use AmendCommit (faster, no graph walk) and
// older commits use RebaseRewrite (first-parent chain rebuild).
func (app *App) UpdateCommit(req EditRequest) (OperationResult, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return OperationResult{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	// Server-side safety check: refuse to rewrite a pushed commit. This mirrors
	// the check inside AmendCommit / RebaseRewrite but is done here first so we
	// never stash the worktree for an operation that is going to be rejected.
	commitHash := plumbing.NewHash(req.Hash)
	if !state.UnpushedHashes[commitHash] {
		return OperationResult{}, gitpkg.ErrCommitNotUnpushed
	}

	// Parse the date string supplied by the frontend (RFC 3339 / ISO 8601).
	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		return OperationResult{}, fmt.Errorf("invalid date %q: %w", req.Date, err)
	}

	opts := gitpkg.AmendOptions{
		Message:     req.Message,
		AuthorName:  req.AuthorName,
		AuthorEmail: req.AuthorEmail,
		Date:        date,
	}

	// Check for dirty working tree. If dirty we must stash before rewriting so
	// that uncommitted changes are not lost or corrupted by the graph rebuild.
	isDirty, err := gitpkg.IsDirty(state)
	if err != nil {
		return OperationResult{}, fmt.Errorf("checking working tree: %w", err)
	}

	var gitBin string
	var stashed bool
	if isDirty {
		// FindGitBinary returns ErrNativeGitNotFound when git is not on PATH.
		// go-git has no stash API, so we cannot proceed without the native binary.
		gitBin, err = gitpkg.FindGitBinary()
		if err != nil {
			return OperationResult{}, err
		}
		if err = gitpkg.AutoStash(state, gitBin); err != nil {
			return OperationResult{}, fmt.Errorf("stashing changes: %w", err)
		}
		stashed = true
	}

	// HEAD rewrites use AmendCommit (no graph walk needed).
	// Older commits use RebaseRewrite (rebuilds the full chain above the target).
	head, err := state.Repo.Head()
	if err != nil {
		return OperationResult{}, fmt.Errorf("reading HEAD: %w", err)
	}

	// rewriteErr captures errors from either rewrite method; we want to report
	// stash pop errors separately since the rewrite may have succeeded but the
	// stash pop failed (leaving the user with a stash that they may not notice
	// if we report it as part of the rewrite error).
	var rewriteErr error
	if head.Hash() == commitHash {
		rewriteErr = gitpkg.AmendCommit(state, opts)
	} else {
		rewriteErr = gitpkg.RebaseRewrite(state, commitHash, opts)
	}

	// Record the pre- and post-rewrite tips so the operation can be undone.
	// This happens before the stash pop so the record exists even when the
	// pop fails, since the rewrite itself still succeeded.
	if rewriteErr == nil {
		if newHead, headErr := state.Repo.Head(); headErr == nil {
			app.mutex.Lock()
			app.lastRewrite = &rewriteRecord{
				RepoPath:   state.Path,
				Branch:     head.Name(),
				BeforeHash: head.Hash(),
				AfterHash:  newHead.Hash(),
			}
			app.mutex.Unlock()
		}
	}

	// Always restore the stash — whether or not the rewrite succeeded — so the
	// user's in-progress work is never left trapped in the stash.
	if stashed {
		if popErr := gitpkg.AutoStashPop(state, gitBin); popErr != nil {
			if rewriteErr != nil {
				// Both failed: report the rewrite error; the stash is still there.
				return OperationResult{}, fmt.Errorf("rewrite failed: %w; also failed to restore stash: %v", rewriteErr, popErr)
			}
			// Rewrite succeeded but pop failed: tell the user explicitly.
			return OperationResult{Success: false, Message: "commit updated but stash pop failed: " + popErr.Error()}, nil
		}
	}

	if rewriteErr != nil {
		return OperationResult{}, rewriteErr
	}

	// Refresh the stored RepoState so subsequent calls (GetCommitLog,
	// GetCommitDetail, etc.) see the new HEAD. Not fatal if it fails.
	if newState, refreshErr := gitpkg.Open(state.Path); refreshErr == nil {
		app.mutex.Lock()
		app.repoState = newState
		app.mutex.Unlock()
	}

	if stashed {
		return OperationResult{Success: true, Message: "commit updated; stashed changes restored"}, nil
	}
	return OperationResult{Success: true, Message: "commit updated"}, nil
}

// UndoLastOperation reverts the most recent successful UpdateCommit by moving
// the branch back to the commit it pointed at before the rewrite. Only one
// level of undo is kept; the record is cleared once it has been used.
//
// Undo is refused (ErrBranchMoved) when the branch has changed since the
// rewrite, e.g. a new commit was made or another branch was checked out.
func (app *App) UndoLastOperation() (OperationResult, error) {
	app.mutex.Lock()
	record := app.lastRewrite
	app.mutex.Unlock()

	if record == nil {
		return OperationResult{}, fmt.Errorf("there is no operation to undo")
	}

	// Re-open the repository so the checks run against the current on-disk
	// state (in-progress operations, detached HEAD, current branch tip).
	state, err := gitpkg.Open(record.RepoPath)
	if err != nil {
		return OperationResult{}, err
	}

	if err := gitpkg.ResetBranch(state, record.Branch, record.AfterHash, record.BeforeHash); err != nil {
		if errors.Is(err, gitpkg.ErrBranchMoved) {
			// The record can never become valid again, so drop it.
			app.mutex.Lock()
			app.lastRewrite = nil
			app.mutex.Unlock()
		}
		return OperationResult{}, err
	}

	app.mutex.Lock()
	app.lastRewrite = nil
	app.mutex.Unlock()

	// Refresh the stored RepoState so subsequent calls see the restored HEAD.
	// Not fatal if it fails.
	if newState, refreshErr := gitpkg.Open(record.RepoPath); refreshErr == nil {
		app.mutex.Lock()
		app.repoState = newState
		app.mutex.Unlock()
	}

	return OperationResult{Success: true, Message: "last rewrite undone"}, nil
}

// CanUndo reports whether UndoLastOperation currently has a rewrite to revert.
// The frontend uses it to re-sync its Undo button after a failed undo.
func (app *App) CanUndo() bool {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	return app.lastRewrite != nil
}
