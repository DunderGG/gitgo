package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	return repoInfoFromState(state), nil
}

// SwitchBranch changes the branch the app operates on to the local branch
// with the given short name. It does not check the branch out: the working
// tree and HEAD are untouched, and later edits move only that branch's ref.
// OpenRepository must be called before this method.
func (app *App) SwitchBranch(branch string) (RepoInfo, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return RepoInfo{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	newState, err := gitpkg.OpenBranch(state.Path, branch)
	if err != nil {
		return RepoInfo{}, err
	}

	// The frontend drops its Undo button on every view change, so drop the
	// backend record too to keep the two in sync.
	app.mutex.Lock()
	app.repoState = newState
	app.lastRewrite = nil
	app.mutex.Unlock()

	return repoInfoFromState(newState), nil
}

// ReloadRepository re-reads the open repository and branch from disk and
// returns up-to-date RepoInfo (upstream, remote and checked-out state can all
// change outside the app). If the branch was deleted in the meantime, it falls
// back to the checked-out branch. The undo record is kept: if the branch moved,
// UndoLastOperation reports that itself.
// OpenRepository must be called before this method.
func (app *App) ReloadRepository() (RepoInfo, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return RepoInfo{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	newState, err := gitpkg.OpenBranch(state.Path, state.Branch)
	if errors.Is(err, gitpkg.ErrBranchNotFound) {
		newState, err = gitpkg.Open(state.Path)
	}
	if err != nil {
		return RepoInfo{}, err
	}

	app.mutex.Lock()
	app.repoState = newState
	app.mutex.Unlock()

	return repoInfoFromState(newState), nil
}

// ListBranches returns the short names of all local branches in the open
// repository, sorted alphabetically.
func (app *App) ListBranches() ([]string, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	return gitpkg.ListBranches(state)
}

// repoInfoFromState maps a git.RepoState to the RepoInfo DTO.
func repoInfoFromState(state *gitpkg.RepoState) RepoInfo {
	return RepoInfo{
		Path:         state.Path,
		Branch:       state.Branch,
		IsCheckedOut: state.IsCheckedOut,
		HasRemote:    state.HasRemote,
		HasUpstream:  state.HasUpstream,
	}
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
		Hash:           commit.Hash.String(),
		Message:        commit.Message,
		AuthorName:     commit.Author.Name,
		AuthorEmail:    commit.Author.Email,
		Date:           commit.Author.When.Format("2006-01-02T15:04:05Z07:00"),
		CommitterName:  commit.Committer.Name,
		CommitterEmail: commit.Committer.Email,
		CommitterDate:  commit.Committer.When.Format("2006-01-02T15:04:05Z07:00"),
		IsUnpushed:     state.UnpushedHashes[commitHash],
	}, nil
}

// RefreshLog re-opens the current repository and branch to pick up any changes (e.g.
// after a commit rewrite) and returns an updated commit list.
// OpenRepository must be called before this method.
func (app *App) RefreshLog() ([]CommitSummary, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	newState, err := gitpkg.OpenBranch(state.Path, state.Branch)
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
// commit.
//
// The rewrite targets the branch selected via OpenRepository / SwitchBranch,
// which does not have to be checked out. Uncommitted changes are left exactly
// as they are, staged or not: rewrites only change commit metadata, never file
// trees, so the index and working tree stay consistent with the moved branch.
//
// The commit is rewritten with RebaseRewrite, which rebuilds the first-parent
// chain above it (just the commit itself when it is the branch tip).
func (app *App) UpdateCommit(req EditRequest) (OperationResult, error) {
	// Parse the date string supplied by the frontend (RFC 3339 / ISO 8601).
	// The offset in the string becomes the commit's time zone. An empty date
	// leaves the zero time, which keeps the original author date.
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = time.Parse(time.RFC3339, req.Date)
		if err != nil {
			return OperationResult{}, fmt.Errorf("invalid date %q: %w", req.Date, err)
		}
	}

	opts := gitpkg.AmendOptions{
		Message:     req.Message,
		AuthorName:  req.AuthorName,
		AuthorEmail: req.AuthorEmail,
		Date:        date,

		SyncCommitterDate: req.SyncCommitterDate,
		MoveBranches:      req.MoveBranches,
	}
	commitHash := plumbing.NewHash(req.Hash)

	return app.runRewrite([]plumbing.Hash{commitHash}, req.MoveBranches, "commit updated", func(state *gitpkg.RepoState) error {
		return gitpkg.RebaseRewrite(state, commitHash, opts)
	})
}

// ShiftCommitDates moves the author date of each unpushed commit in req by
// req.Minutes, in a single rewrite that one undo reverts. Each commit keeps
// its own time zone offset; see UpdateCommit for what else is left alone.
func (app *App) ShiftCommitDates(req ShiftRequest) (OperationResult, error) {
	hashes := make([]plumbing.Hash, len(req.Hashes))
	for i, hash := range req.Hashes {
		hashes[i] = plumbing.NewHash(hash)
	}
	opts := gitpkg.ShiftOptions{
		Shift:          time.Duration(req.Minutes) * time.Minute,
		ShiftCommitter: req.ShiftCommitter,
		MoveBranches:   req.MoveBranches,
	}

	message := fmt.Sprintf("%d commits shifted", len(hashes))
	if len(hashes) == 1 {
		message = "1 commit shifted"
	}
	return app.runRewrite(hashes, req.MoveBranches, message, func(state *gitpkg.RepoState) error {
		return gitpkg.ShiftDates(state, hashes, opts)
	})
}

// runRewrite runs a history rewrite of the given commits against a freshly
// read repository state, then records it for undo and refreshes the stored
// state. successMessage is returned when everything, including moving
// moveBranches, succeeded.
func (app *App) runRewrite(hashes []plumbing.Hash, moveBranches []string, successMessage string, rewrite func(state *gitpkg.RepoState) error) (OperationResult, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return OperationResult{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}
	if len(hashes) == 0 {
		return OperationResult{}, fmt.Errorf("no commits selected")
	}

	// Re-read the repository before the safety check: the state was computed
	// when the view was loaded, and commits may have been pushed (or the branch
	// moved) from a terminal since then. The fresh state is kept so the UI's
	// next refresh reflects it even when the edit is rejected.
	state, err := gitpkg.OpenBranch(state.Path, state.Branch)
	if err != nil {
		return OperationResult{}, err
	}
	app.mutex.Lock()
	app.repoState = state
	app.mutex.Unlock()

	// Server-side safety check: refuse to rewrite a pushed commit. This mirrors
	// the check inside the git layer but is done here first so a rejected edit
	// fails before any other work.
	for _, hash := range hashes {
		if !state.UnpushedHashes[hash] {
			return OperationResult{}, gitpkg.ErrCommitNotUnpushed
		}
	}

	branchRefName := plumbing.NewBranchReferenceName(state.Branch)
	tip, err := state.Repo.Reference(branchRefName, true)
	if err != nil {
		return OperationResult{}, fmt.Errorf("reading branch %s: %w", state.Branch, err)
	}

	// Note where the other branches point, so the ones the rewrite moves can
	// be moved back by undo.
	otherTipsBefore := branchTips(state, moveBranches)

	rewriteErr := rewrite(state)

	// Some requested branches could not be moved, but the edit itself stands:
	// treat it as a success with a warning.
	var refsNotMoved *gitpkg.RefsNotMovedError
	if errors.As(rewriteErr, &refsNotMoved) {
		rewriteErr = nil
	}
	if rewriteErr != nil {
		return OperationResult{}, rewriteErr
	}

	// Record the pre- and post-rewrite tips so the operation can be undone.
	if newTip, tipErr := state.Repo.Reference(branchRefName, true); tipErr == nil {
		app.mutex.Lock()
		app.lastRewrite = &rewriteRecord{
			RepoPath:      state.Path,
			Branch:        branchRefName,
			BeforeHash:    tip.Hash(),
			AfterHash:     newTip.Hash(),
			MovedBranches: movedBranches(state, otherTipsBefore),
		}
		app.mutex.Unlock()
	}

	// Refresh the stored RepoState so subsequent calls (GetCommitLog,
	// GetCommitDetail, etc.) see the new HEAD. Not fatal if it fails.
	if newState, refreshErr := gitpkg.OpenBranch(state.Path, state.Branch); refreshErr == nil {
		app.mutex.Lock()
		app.repoState = newState
		app.mutex.Unlock()
	}

	if refsNotMoved != nil {
		return OperationResult{Success: false, Message: refsNotMoved.Error()}, nil
	}
	return OperationResult{Success: true, Message: successMessage}, nil
}

// GetAffectedRefs lists the other branches and tags that an edit of the given
// commits would leave pointing at old commits, for the confirm dialog.
func (app *App) GetAffectedRefs(hashes []string) ([]AffectedRef, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	targets := make([]plumbing.Hash, len(hashes))
	for i, hash := range hashes {
		targets[i] = plumbing.NewHash(hash)
	}
	refs, err := gitpkg.FindAffectedRefs(state, targets...)
	if err != nil {
		return nil, err
	}
	result := make([]AffectedRef, 0, len(refs))
	for _, ref := range refs {
		result = append(result, AffectedRef{Name: ref.Name, Kind: string(ref.Kind)})
	}
	return result, nil
}

// branchTips returns the current tip of each named local branch that exists.
func branchTips(state *gitpkg.RepoState, names []string) map[plumbing.ReferenceName]plumbing.Hash {
	tips := make(map[plumbing.ReferenceName]plumbing.Hash, len(names))
	for _, name := range names {
		refName := plumbing.NewBranchReferenceName(name)
		if ref, err := state.Repo.Reference(refName, true); err == nil {
			tips[refName] = ref.Hash()
		}
	}
	return tips
}

// movedBranches compares the branches in before with where they point now and
// returns the ones that moved.
func movedBranches(state *gitpkg.RepoState, before map[plumbing.ReferenceName]plumbing.Hash) []movedBranch {
	var moved []movedBranch
	for refName, beforeHash := range before {
		ref, err := state.Repo.Reference(refName, true)
		if err != nil || ref.Hash() == beforeHash {
			continue
		}
		moved = append(moved, movedBranch{Branch: refName, BeforeHash: beforeHash, AfterHash: ref.Hash()})
	}
	return moved
}

// UndoLastOperation reverts the most recent successful UpdateCommit by moving
// the branch back to the commit it pointed at before the rewrite. Only one
// level of undo is kept; the record is cleared once it has been used.
//
// Undo is refused (ErrBranchMoved) when the branch has changed since the
// rewrite, e.g. a new commit was made or the branch was deleted.
func (app *App) UndoLastOperation() (OperationResult, error) {
	app.mutex.Lock()
	record := app.lastRewrite
	app.mutex.Unlock()

	if record == nil {
		return OperationResult{}, fmt.Errorf("there is no operation to undo")
	}

	// Re-open the rewritten branch so the checks run against the current
	// on-disk state (in-progress operations, detached HEAD, branch tip, and
	// which commits have been pushed since the rewrite).
	state, err := gitpkg.OpenBranch(record.RepoPath, record.Branch.Short())
	if errors.Is(err, gitpkg.ErrBranchNotFound) {
		// The branch was deleted; treat it like any other move.
		err = gitpkg.ErrBranchMoved
	}
	if err == nil {
		err = gitpkg.ResetBranch(state, record.Branch, record.AfterHash, record.BeforeHash)
	}
	if err != nil {
		if errors.Is(err, gitpkg.ErrBranchMoved) || errors.Is(err, gitpkg.ErrUndoPushed) {
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

	// Move the branches that were moved along with the edit back as well. The
	// edited branch is already restored, so failures here are only reported.
	var notRestored []string
	for _, moved := range record.MovedBranches {
		movedState, openErr := gitpkg.OpenBranch(record.RepoPath, moved.Branch.Short())
		if openErr == nil {
			openErr = gitpkg.ResetBranch(movedState, moved.Branch, moved.AfterHash, moved.BeforeHash)
		}
		if openErr != nil {
			notRestored = append(notRestored, moved.Branch.Short())
		}
	}

	// Refresh the stored RepoState so subsequent calls see the restored HEAD.
	// Not fatal if it fails.
	if newState, refreshErr := gitpkg.OpenBranch(record.RepoPath, record.Branch.Short()); refreshErr == nil {
		app.mutex.Lock()
		app.repoState = newState
		app.mutex.Unlock()
	}

	if len(notRestored) > 0 {
		return OperationResult{
			Success: false,
			Message: "last rewrite undone, but branch " + strings.Join(notRestored, ", ") + " changed since the edit and was not moved back",
		}, nil
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
