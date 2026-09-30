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
	// SyncCommitterDate sets the committer date to the new author date. When
	// false the committer date is kept.
	SyncCommitterDate bool
	// Committer says what happens to the committer name and email; with
	// SetCommitter they become CommitterName and CommitterEmail.
	Committer      CommitterChange
	CommitterName  string
	CommitterEmail string
	// MoveBranches names other local branches to move along with the edit:
	// each one that points at a rewritten commit is moved to its new copy
	// (see FindAffectedRefs). Others are ignored.
	MoveBranches []string
}

// BulkEditOptions describes the same change applied to several commits (see
// EditCommits): a date shift, a new author, a new committer, or a mix.
type BulkEditOptions struct {
	// Shift is added to each commit's author date. Zero keeps the dates.
	Shift time.Duration
	// ShiftCommitter also adds Shift to each commit's committer date. When
	// false the committer dates are kept.
	ShiftCommitter bool
	// SetAuthor replaces each commit's author name and email with AuthorName
	// and AuthorEmail. The author dates are only changed by Shift.
	SetAuthor   bool
	AuthorName  string
	AuthorEmail string
	// Committer says what happens to each commit's committer name and email,
	// as in AmendOptions. The committer dates are only changed by Shift.
	Committer      CommitterChange
	CommitterName  string
	CommitterEmail string
	// MoveBranches names other local branches to move along with the
	// rewrite, as in AmendOptions.
	MoveBranches []string
}

// CommitterChange says what an edit does with a commit's committer name and
// email. The committer date is handled separately.
type CommitterChange int

const (
	// KeepCommitter leaves the committer name and email unchanged.
	KeepCommitter CommitterChange = iota
	// SetCommitter replaces them with the given name and email.
	SetCommitter
	// CommitterFromAuthor copies the commit's author name and email, after
	// the edit's own author change, so the two match again.
	CommitterFromAuthor
)

// Identity is a name and email as used in a commit's author or committer.
type Identity struct {
	Name  string
	Email string
}

// CommitEntry is the git-layer representation of a single commit.
// The app layer converts this to app.CommitSummary.
type CommitEntry struct {
	Hash          plumbing.Hash
	ShortHash     string
	Message       string
	AuthorName    string
	CommitterName string
	Date          time.Time
	IsUnpushed    bool
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

	// IsCheckedOut is true when Branch is the branch HEAD points at. Rewrites
	// never touch the working tree either way, since file trees are unchanged.
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
