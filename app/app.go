package app

import (
	"context"
	"fmt"
	"sync"

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
