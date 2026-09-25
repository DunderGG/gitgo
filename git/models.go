package git

import (
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// AmendOptions holds the new metadata values for a commit being amended.
type AmendOptions struct {
	// Message is the full commit message, including a trailing newline.
	Message     string
	AuthorName  string
	AuthorEmail string
	// Date is the new author date, including its time zone offset. The zero
	// value keeps the commit's original author date unchanged.
	Date time.Time
	// SyncCommitterDate sets the committer date to the new author date. The
	// committer name and email are always kept, and so is the committer date
	// when this is false.
	SyncCommitterDate bool
	// MoveBranches names other local branches to move along with the edit:
	// each one that points at a rewritten commit is moved to its new copy
	// (see FindAffectedRefs). Others are ignored.
	MoveBranches []string
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

	// Branch is the short name of the branch this state targets. Log, rewrite
	// and undo operate on this branch's ref, not on HEAD.
	Branch string

	// IsCheckedOut is true when Branch is the branch HEAD points at. Only then
	// can a rewrite interact with the working tree (auto-stash).
	IsCheckedOut bool

	// HasRemote is true when at least one remote is configured.
	HasRemote bool

	// HasUpstream is true when the current branch has a remote tracking branch.
	HasUpstream bool

	// UnpushedHashes is the set of commit hashes reachable from the branch tip
	// but not from its upstream or any other remote-tracking ref (i.e. safe to
	// edit). All commits are considered unpushed when there are no
	// remote-tracking refs.
	UnpushedHashes map[plumbing.Hash]bool
}
