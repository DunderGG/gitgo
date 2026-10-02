package app

import (
	"fmt"
	"strings"
	"time"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// Backups save branch tips as refs under refs/gitgo/backups/ (see
// git/backup.go and docs/BACKUPS.md), so a branch can be restored after any
// number of edits, also after the app was closed.

// CreateBackup saves the current tip of the open branch as a manual backup.
func (app *App) CreateBackup() (BackupInfo, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return BackupInfo{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	state, err := gitpkg.OpenBranch(state.Path, state.Branch)
	if err != nil {
		return BackupInfo{}, err
	}
	backup, err := gitpkg.CreateBackup(state, gitpkg.BackupManual, nil)
	if err != nil {
		return BackupInfo{}, err
	}
	return backupInfo(state, backup), nil
}

// ListBackups returns every backup in the open repository, newest first. The
// frontend filters them by branch.
func (app *App) ListBackups() ([]BackupInfo, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	backups, err := gitpkg.ListBackups(state.Repo)
	if err != nil {
		return nil, err
	}
	result := make([]BackupInfo, len(backups))
	for i, backup := range backups {
		result[i] = backupInfo(state, backup)
	}
	return result, nil
}

// DeleteBackup removes the backup with the given ID.
func (app *App) DeleteBackup(id string) error {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return fmt.Errorf("no repository is open; call OpenRepository first")
	}
	return gitpkg.DeleteBackup(state.Repo, id)
}

// PlanRestore says what restoring the backup with the given ID would do, for
// the confirm dialog. Pass each branch's CurrentTip back to RestoreBackup.
func (app *App) PlanRestore(id string) (RestorePlanInfo, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return RestorePlanInfo{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}

	plan, err := gitpkg.PlanRestore(state.Path, id)
	if err != nil {
		return RestorePlanInfo{}, err
	}
	return restorePlanInfo(state, plan), nil
}

// RestoreBackup moves the backup's branches back to the commits it saved.
// expectedTips maps each branch to the current tip shown by PlanRestore; if a
// branch has moved since, nothing is changed.
//
// The branches' current tips are saved as an automatic backup first, so a
// restore can itself be restored, and the restore is recorded for undo like
// an edit.
func (app *App) RestoreBackup(id string, expectedTips map[string]string) (OperationResult, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return OperationResult{}, fmt.Errorf("no repository is open; call OpenRepository first")
	}
	path := state.Path

	expected := make(map[string]plumbing.Hash, len(expectedTips))
	for branch, hash := range expectedTips {
		expected[branch] = plumbing.NewHash(hash)
	}

	// Check before saving anything, so a refused restore leaves no backup.
	plan, err := gitpkg.PlanRestore(path, id)
	if err != nil {
		return OperationResult{}, err
	}
	var moving []string
	for _, branch := range plan.Branches {
		if branch.Missing || branch.Unchanged() {
			continue
		}
		if want, ok := expected[branch.Branch]; !ok || want != branch.CurrentTip {
			return OperationResult{}, gitpkg.ErrBranchChanged
		}
		moving = append(moving, branch.Branch)
	}
	if err := plan.Err(); err != nil {
		return OperationResult{}, err
	}
	if len(moving) == 0 {
		return OperationResult{Success: true, Message: "the branches already match this backup"}, nil
	}

	backupState, err := gitpkg.OpenBranch(path, moving[0])
	if err != nil {
		return OperationResult{}, err
	}
	saved, err := gitpkg.CreateBackup(backupState, gitpkg.BackupAuto, moving[1:])
	if err != nil {
		return OperationResult{}, fmt.Errorf("saving the current state before restoring: %w", err)
	}

	done, restoreErr := gitpkg.RestoreBackup(path, id, expected)
	if done.Branches == nil {
		// Nothing was moved; the backup of the current state is not needed.
		_ = gitpkg.DeleteBackup(backupState.Repo, saved.ID)
		return OperationResult{}, restoreErr
	}

	app.recordRestore(path, state.Branch, done)

	if newState, refreshErr := gitpkg.OpenBranch(path, state.Branch); refreshErr == nil {
		app.mutex.Lock()
		app.repoState = newState
		app.mutex.Unlock()
	}

	if restoreErr != nil {
		return OperationResult{Success: false, Message: restoreErr.Error()}, nil
	}
	return OperationResult{Success: true, Message: "backup restored; the previous state was saved as a backup"}, nil
}

// recordRestore records the branches a restore moved so Ctrl+Z can move them
// back. The viewed branch is the main one when it moved.
func (app *App) recordRestore(path, viewed string, plan gitpkg.RestorePlan) {
	var moved []movedBranch
	for _, branch := range plan.Branches {
		if branch.Missing || branch.Unchanged() {
			continue
		}
		refName := plumbing.NewBranchReferenceName(branch.Branch)
		state, err := gitpkg.OpenBranch(path, branch.Branch)
		if err != nil {
			continue
		}
		if tip, err := state.Repo.Reference(refName, true); err != nil || tip.Hash() != branch.BackupTip {
			// Not moved (see the partial failure in RestoreBackup).
			continue
		}
		entry := movedBranch{Branch: refName, BeforeHash: branch.CurrentTip, AfterHash: branch.BackupTip}
		if branch.Branch == viewed {
			moved = append([]movedBranch{entry}, moved...)
		} else {
			moved = append(moved, entry)
		}
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()
	if len(moved) == 0 {
		app.lastRewrite = nil
		return
	}
	app.lastRewrite = &rewriteRecord{
		RepoPath:      path,
		Branch:        moved[0].Branch,
		BeforeHash:    moved[0].BeforeHash,
		AfterHash:     moved[0].AfterHash,
		MovedBranches: moved[1:],
	}
}

// backupInfo maps a git.Backup to its DTO, noting for each branch whether it
// still exists and already points at the saved commit.
func backupInfo(state *gitpkg.RepoState, backup gitpkg.Backup) BackupInfo {
	info := BackupInfo{
		ID:       backup.ID,
		Kind:     string(backup.Kind),
		Created:  backup.Created.Format(time.RFC3339),
		Branches: make([]BackupBranchInfo, len(backup.Branches)),
	}
	for i, saved := range backup.Branches {
		branch := BackupBranchInfo{
			Name:      saved.Name,
			Hash:      saved.Hash.String(),
			ShortHash: saved.Hash.String()[:7],
		}
		if commit, err := state.Repo.CommitObject(saved.Hash); err == nil {
			branch.Subject, _, _ = strings.Cut(commit.Message, "\n")
		}
		ref, err := state.Repo.Reference(plumbing.NewBranchReferenceName(saved.Name), true)
		if err == nil {
			branch.Exists = true
			branch.Current = ref.Hash() == saved.Hash
		}
		info.Branches[i] = branch
	}
	return info
}

// restorePlanInfo maps a git.RestorePlan to its DTO.
func restorePlanInfo(state *gitpkg.RepoState, plan gitpkg.RestorePlan) RestorePlanInfo {
	info := RestorePlanInfo{
		Backup:   backupInfo(state, plan.Backup),
		Branches: make([]RestoreBranchInfo, len(plan.Branches)),
	}
	if err := plan.Err(); err != nil {
		info.Problem = err.Error()
	}
	for i, branch := range plan.Branches {
		entry := RestoreBranchInfo{
			Name:          branch.Branch,
			Missing:       branch.Missing,
			Unchanged:     branch.Unchanged(),
			CheckedOut:    branch.CheckedOut,
			Removed:       restoreCommitInfos(branch.Removed),
			RemovedCount:  branch.RemovedCount,
			Returned:      restoreCommitInfos(branch.Returned),
			ReturnedCount: branch.ReturnedCount,
		}
		if !branch.Missing {
			entry.CurrentTip = branch.CurrentTip.String()
		}
		if branch.Blocked != nil {
			entry.Problem = branch.Blocked.Error()
		}
		info.Branches[i] = entry
	}
	return info
}

// restoreCommitInfos maps commits listed in a restore plan to their DTOs.
func restoreCommitInfos(commits []gitpkg.RestoreCommit) []RestoreCommitInfo {
	infos := make([]RestoreCommitInfo, len(commits))
	for i, commit := range commits {
		infos[i] = RestoreCommitInfo{
			Hash:      commit.Hash.String(),
			ShortHash: commit.Hash.String()[:7],
			Subject:   commit.Subject,
		}
	}
	return infos
}
