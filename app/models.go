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
	// opened or closed, or the rewrite is undone. Guarded by mutex.
	lastRewrite *rewriteRecord

	// settingsPath is the settings file (app/settings.go), or empty when there
	// is no config directory. settingsMutex serialises its read-modify-write.
	settingsPath  string
	settingsMutex sync.Mutex
}

// Settings are the user's preferences, kept in settings.json in GitGo's config
// directory.
type Settings struct {
	// RecentRepos are the repository paths opened most recently, newest first.
	RecentRepos []string `json:"recentRepos"`
	// Theme is the colour theme: ThemeSystem, ThemeLight or ThemeDark.
	Theme string `json:"theme"`
	// SubjectGuide is the column of the subject ruler in the commit message
	// field, and the longest subject before a hint. BodyGuide is the longest
	// body line before a hint. 0 turns a guide off.
	SubjectGuide int `json:"subjectGuide"`
	BodyGuide    int `json:"bodyGuide"`
	// TerminalCommand is the command the terminal button runs, with {dir} for
	// the repository folder (for example `wt.exe -d {dir} pwsh`). Empty picks
	// a terminal automatically.
	TerminalCommand string `json:"terminalCommand"`
	// OfficeHours are the user's working hours, for spreading commits with
	// "Only office hours".
	OfficeHours OfficeHours `json:"officeHours"`
}

// OfficeHours is a daily window of working time on chosen weekdays.
type OfficeHours struct {
	// Start and End are times of day as "HH:MM", with Start before End.
	Start string `json:"start"`
	End   string `json:"end"`
	// Days are the working days, 0 for Sunday to 6 for Saturday.
	Days []int `json:"days"`
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
	Committer  string `json:"committer"`
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
	// Committer fields show the current committer; EditRequest says whether
	// an edit keeps or changes them.
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
	// Committer is what happens to the committer name and email: "keep" (or
	// empty), "author" to copy the new author, or "set" to use CommitterName
	// and CommitterEmail. The committer date is handled by SyncCommitterDate.
	Committer      string `json:"committer"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	// MoveBranches names other local branches (from GetAffectedRefs) to move
	// to the rewritten commits along with the edited branch.
	MoveBranches []string `json:"moveBranches"`
}

// BulkEditRequest describes the same change applied to several unpushed
// commits: a date shift, a new author, a new committer, or a mix.
type BulkEditRequest struct {
	Hashes []string `json:"hashes"`
	// Minutes is added to each commit's author date; negative moves it
	// earlier, zero keeps the dates.
	Minutes int `json:"minutes"`
	// Dates sets each commit's author date, by hash, as RFC 3339 (the dates
	// returned by SpreadDates). It needs every hash and cannot be combined
	// with Minutes. Empty keeps the dates (or applies Minutes).
	Dates map[string]string `json:"dates"`
	// ShiftCommitter moves each commit's committer date by as much as its
	// author date moved. When false the committer dates are kept.
	ShiftCommitter bool `json:"shiftCommitter"`
	// SetAuthor replaces each commit's author name and email with
	// AuthorName and AuthorEmail. When false the authors are kept.
	SetAuthor   bool   `json:"setAuthor"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	// Committer is what happens to each commit's committer name and email,
	// as in EditRequest ("author" copies each commit's new author). The
	// committer dates are handled by ShiftCommitter.
	Committer      string `json:"committer"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	// MoveBranches names other local branches (from GetAffectedRefs) to move
	// to the rewritten commits along with the edited branch.
	MoveBranches []string `json:"moveBranches"`
}

// SpreadRequest asks SpreadDates for new author dates that fit several
// unpushed commits between a first and last date.
type SpreadRequest struct {
	Hashes []string `json:"hashes"`
	// First and Last are RFC 3339 dates for the oldest and newest commit.
	First string `json:"first"`
	Last  string `json:"last"`
	// Spacing is "keep" to scale the current gaps between the commits,
	// "even" for equal gaps, or "random" for random gaps of at least
	// MinGapMinutes, repeatable with Seed.
	Spacing       string `json:"spacing"`
	MinGapMinutes int    `json:"minGapMinutes"`
	Seed          uint32 `json:"seed"`
	// OfficeHours, when set, keeps the commits within them, read in First's
	// time zone; First and Last must be within them. Nil uses all the time.
	OfficeHours *OfficeHours `json:"officeHours"`
}

// SpreadResult is the outcome of SpreadDates.
type SpreadResult struct {
	// Dates is each commit's new author date by hash, as RFC 3339 in the
	// commit's own offset; pass it on as BulkEditRequest.Dates.
	Dates map[string]string `json:"dates"`
	// FellBack is set when "keep" was asked for but the commits were spaced
	// evenly, because their current dates are all equal or out of order.
	FellBack bool `json:"fellBack"`
}

// Identity is the author name and email git is configured to use for new
// commits, for the "Use my identity" buttons. Either field may be empty.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
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
