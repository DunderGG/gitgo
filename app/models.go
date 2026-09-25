package app

import (
	"context"
	gitpkg "gitgo/git"
	"sync"

	"github.com/go-git/go-git/v5/plumbing"
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

	// lastRewrite records the most recent successful rewrite so it can be
	// undone. It lives in memory only and is cleared when a repository is
	// opened or the rewrite is undone. Guarded by mutex.
	lastRewrite *rewriteRecord
}

// rewriteRecord captures the branch tip before and after a rewrite. Undo moves
// the branch from AfterHash back to BeforeHash, and does the same for every
// other branch that was moved along with the edit.
type rewriteRecord struct {
	RepoPath      string
	Branch        plumbing.ReferenceName
	BeforeHash    plumbing.Hash
	AfterHash     plumbing.Hash
	MovedBranches []movedBranch
}

// movedBranch is another branch moved by a rewrite (EditRequest.MoveBranches).
type movedBranch struct {
	Branch     plumbing.ReferenceName
	BeforeHash plumbing.Hash
	AfterHash  plumbing.Hash
}

// RepoInfo holds high-level information about the currently opened repository.
// This is returned by OpenRepository and used to populate the repository info panel.
type RepoInfo struct {
	Path         string `json:"path"`
	Branch       string `json:"branch"`
	IsCheckedOut bool   `json:"isCheckedOut"`
	HasRemote    bool   `json:"hasRemote"`
	HasUpstream  bool   `json:"hasUpstream"`
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
	// Committer fields are read-only in the edit panel; rewrites keep them
	// unless the committer date is synced to the author date.
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	CommitterDate  string `json:"committerDate"`
	IsUnpushed     bool   `json:"isUnpushed"`
}

// EditRequest describes the desired change to a commit's metadata.
// This is returned by the frontend when the user submits changes in the edit panel.
type EditRequest struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	// Date is the new author date as RFC 3339 with the desired offset, or
	// empty to keep the original author date.
	Date string `json:"date"`
	// SyncCommitterDate sets the committer date to the (new) author date. When
	// false the original committer date is kept.
	SyncCommitterDate bool `json:"syncCommitterDate"`
	// MoveBranches names other local branches (from GetAffectedRefs) to move
	// to the rewritten commits along with the edited branch.
	MoveBranches []string `json:"moveBranches"`
}

// ShiftRequest describes a date shift applied to several unpushed commits.
type ShiftRequest struct {
	Hashes []string `json:"hashes"`
	// Minutes is added to each commit's author date; negative moves it
	// earlier. It must not be zero.
	Minutes int `json:"minutes"`
	// ShiftCommitter also shifts each commit's committer date by Minutes.
	// When false the committer dates are kept.
	ShiftCommitter bool `json:"shiftCommitter"`
	// MoveBranches names other local branches (from GetAffectedRefs) to move
	// to the rewritten commits along with the edited branch.
	MoveBranches []string `json:"moveBranches"`
}

// AffectedRef is a branch or tag that an edit would leave pointing at old
// commits. Kind is "branch" (can be moved with the edit), "forked-branch"
// (has its own commits on top of a rewritten commit) or "tag" (never moved).
type AffectedRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// SignedCommit is a signed commit that an edit would leave unsigned, for the
// confirm dialog's warning. Edited is false for commits that are only
// rebuilt because they sit above an edited one.
type SignedCommit struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Subject   string `json:"subject"`
	Edited    bool   `json:"edited"`
}

// OperationResult is returned by all mutating bound methods to convey
// success or failure to the frontend.
// Note that even if Success is true, the Message may contain warnings or other
// non-error information that the frontend may want to display to the user.
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
