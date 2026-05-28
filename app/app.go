package app

import (
	"context"
	"fmt"
	"sync"

	gitpkg "gitgo/git"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct. It is bound to the Wails runtime and
// exposes methods to the frontend via IPC. All bound methods must return either
// a single value or (T, error) to satisfy the Wails binding contract.
type App struct {
	ctx       context.Context
	mu        sync.Mutex
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

	application.mu.Lock()
	application.repoState = state
	application.mu.Unlock()

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
	application.mu.Lock()
	state := application.repoState
	application.mu.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	entries, err := gitpkg.Log(state, 0)
	if err != nil {
		return nil, err
	}

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

	return summaries, nil
}
