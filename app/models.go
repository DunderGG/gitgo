package app

import (
	"context"
	gitpkg "gitgo/git"
	"sync"
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

// RepoInfo holds high-level information about the currently opened repository.
// This is returned by OpenRepository and used to populate the repository info panel.
type RepoInfo struct {
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	HasRemote   bool   `json:"hasRemote"`
	HasUpstream bool   `json:"hasUpstream"`
}

// CommitSummary is a lightweight representation of a commit for the commit list.
// IsUnpushed indicates whether the commit is safe to edit.
type CommitSummary struct {
	Hash       string `json:"hash"`
	ShortHash  string `json:"shortHash"`
	Message    string `json:"message"`
	Author     string `json:"author"`
	Date       string `json:"date"`
	IsUnpushed bool   `json:"isUnpushed"`
}

// CommitDetail carries the full metadata needed to populate the edit panel.
// This is returned by GetCommitDetail and used to populate the edit panel.
type CommitDetail struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Date        string `json:"date"`
	IsUnpushed  bool   `json:"isUnpushed"`
}

// EditRequest describes the desired change to a commit's metadata.
// This is returned by the frontend when the user submits changes in the edit panel.
type EditRequest struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Date        string `json:"date"`
}

// OperationResult is returned by all mutating bound methods to convey
// success or failure to the frontend.
// Note that even if Success is true, the Message may contain warnings or other
// non-error information that the frontend may want to display to the user.
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

