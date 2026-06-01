package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct. It is bound to the Wails runtime and
// exposes methods to the frontend via IPC. All bound methods must return either
// a single value or (T, error) to satisfy the Wails binding contract.
type App struct {
	ctx context.Context

	// mutex guards repoState. Bound methods are called from the WebView's IPC
	// goroutine, which is separate from the Go main goroutine, so concurrent
	// access to repoState is possible even with a single user (e.g. the UI may
	// auto-call GetCommitLog before OpenRepository has finished writing the
	// pointer). The mutex keeps the Go race detector clean and satisfies the Go
	// memory model without any real performance cost — contention never occurs
	// in practice.
	mutex     sync.Mutex
	repoState *gitpkg.RepoState
}

// New creates a new App instance.
func New() *App {
	return &App{}
}

// Startup is called when the Wails application starts and stores the context
// for later use by bound methods.
func (application *App) Startup(ctx context.Context) {
	application.ctx = ctx
}

// SelectDirectory opens a native directory picker dialog and returns the
// selected path. Returns an empty string if the user cancels.
func (application *App) SelectDirectory() (string, error) {
	path, err := runtime.OpenDirectoryDialog(application.ctx, runtime.OpenDialogOptions{
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
func (application *App) OpenRepository(path string) (RepoInfo, error) {
	state, err := gitpkg.Open(path)
	if err != nil {
		return RepoInfo{}, err
	}

	application.mutex.Lock()
	application.repoState = state
	application.mutex.Unlock()

	return RepoInfo{
		Path:        state.Path,
		Branch:      state.Branch,
		HasRemote:   state.HasRemote,
		HasUpstream: state.HasUpstream,
	}, nil
}

// GetCommitLog returns the commit history for the currently open repository.
// OpenRepository must be called before this method.
func (application *App) GetCommitLog() ([]CommitSummary, error) {
	application.mutex.Lock()
	state := application.repoState
	application.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	entries, err := gitpkg.Log(state, 0)
	if err != nil {
		return nil, err
	}

	return commitSummariesFromEntries(entries), nil
}

// GetCommitDetail returns full metadata for a single commit identified by its
// 40-character hex hash. This is used to populate the edit panel.
func (application *App) GetCommitDetail(hash string) (CommitDetail, error) {
	application.mutex.Lock()
	state := application.repoState
	application.mutex.Unlock()

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
func (application *App) RefreshLog() ([]CommitSummary, error) {
	application.mutex.Lock()
	state := application.repoState
	application.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	newState, err := gitpkg.Open(state.Path)
	if err != nil {
		return nil, err
	}

	application.mutex.Lock()
	application.repoState = newState
	application.mutex.Unlock()

	entries, err := gitpkg.Log(newState, 0)
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

// UpdateCommit applies the metadata changes in req to the identified unpushed
// commit. If the working tree is dirty, changes are automatically stashed
// before the rewrite and restored afterwards.
//
// Under the hood, HEAD rewrites use AmendCommit (faster, no graph walk) and
// older commits use RebaseRewrite (first-parent chain rebuild).
func (application *App) UpdateCommit(req EditRequest) (OperationResult, error) {
	application.mutex.Lock()
	state := application.repoState
	application.mutex.Unlock()

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

	var rewriteErr error
	if head.Hash() == commitHash {
		rewriteErr = gitpkg.AmendCommit(state, opts)
	} else {
		rewriteErr = gitpkg.RebaseRewrite(state, commitHash, opts)
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
		application.mutex.Lock()
		application.repoState = newState
		application.mutex.Unlock()
	}

	if stashed {
		return OperationResult{Success: true, Message: "commit updated; stashed changes restored"}, nil
	}
	return OperationResult{Success: true, Message: "commit updated"}, nil
}
