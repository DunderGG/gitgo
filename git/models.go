package git

import (
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// AmendOptions holds the new metadata values for a commit being amended.
// All fields are required; partial updates are not supported.
type AmendOptions struct {
	// Message is the full commit message, including a trailing newline.
	Message     string
	AuthorName  string
	AuthorEmail string
	Date        time.Time
}

// CommitEntry is the git-layer representation of a single commit.
// The app layer converts this to app.CommitSummary.
type CommitEntry struct {
	Hash       plumbing.Hash
	ShortHash  string
	Message    string
	AuthorName string
	Date       time.Time
	IsUnpushed bool
}

// RepoState holds an open repository and all computed metadata needed by the
// log and rewrite layers.
type RepoState struct {
	// Repo is the underlying go-git repository handle.
	Repo *gogit.Repository

	// Path is the absolute path to the repository root (the working tree).
	Path string

	// Branch is the name of the currently checked-out branch.
	Branch string

	// HasRemote is true when at least one remote is configured.
	HasRemote bool

	// HasUpstream is true when the current branch has a remote tracking branch.
	HasUpstream bool

	// UnpushedHashes is the set of commit hashes that are strictly above the
	// remote tracking tip (i.e. safe to edit). All commits are considered
	// unpushed when there is no upstream.
	UnpushedHashes map[plumbing.Hash]bool
}
