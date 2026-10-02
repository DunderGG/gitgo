package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	gitpkg "gitgo/git"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	app.pruneAutoBackups(backupState.Repo, saved)

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

// ExportBackup writes the backup with the given ID to a git bundle file picked
// in a native save dialog, so it can be kept outside the repository. It
// returns the path written, or an empty string when the dialog is cancelled.
// go-git cannot write bundles, so this needs the native git program, like the
// Run menu.
//
// A full bundle holds each branch's whole history, so it can restore them in
// any repository: `git fetch <file> 'refs/gitgo/backups/*:refs/gitgo/backups/*'`.
// With partial, it leaves out the commits on any remote-tracking branch: a
// much smaller file, which can only be fetched into a clone that already has
// those commits.
func (app *App) ExportBackup(id string, partial bool) (string, error) {
	state, err := app.openState()
	if err != nil {
		return "", err
	}
	backup, err := gitpkg.FindBackup(state.Repo, id)
	if err != nil {
		return "", err
	}
	// Check before asking for a file, so an empty partial bundle is refused
	// without a pointless dialog.
	if partial {
		if err := checkPartialBundle(state.Path, backup); err != nil {
			return "", err
		}
	}
	path, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "Export backup",
		DefaultFilename: bundleFileName(state.Path, backup, partial),
		Filters:         []runtime.FileFilter{{DisplayName: "Git bundle (*.bundle)", Pattern: "*.bundle"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := exportBackup(state.Path, backup, path, partial); err != nil {
		return "", err
	}
	return path, nil
}

// errEmptyPartialBundle is returned for a partial export when every commit of
// the backup is already on a remote; git refuses to write an empty bundle.
var errEmptyPartialBundle = errors.New("every commit in this backup is already on a remote, so a bundle of only the other commits would be empty; export the whole history instead")

// backupRefs returns the full ref names of backup, one per branch.
func backupRefs(backup gitpkg.Backup) []string {
	refs := make([]string, len(backup.Branches))
	for i, branch := range backup.Branches {
		refs[i] = gitpkg.BackupRefPrefix + backup.ID + "/" + branch.Name
	}
	return refs
}

// checkPartialBundle returns errEmptyPartialBundle when a partial bundle of
// backup would hold no commits.
func checkPartialBundle(repoPath string, backup gitpkg.Backup) error {
	args := append([]string{"rev-list", "--count"}, backupRefs(backup)...)
	args = append(args, "--not", "--remotes")
	output, err := gitCommand(repoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rev-list failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) == "0" {
		return errEmptyPartialBundle
	}
	return nil
}

// exportBackup runs `git bundle create` for every ref of backup in the
// repository at repoPath, writing the bundle to file. With partial, commits
// on remote-tracking branches are left out.
func exportBackup(repoPath string, backup gitpkg.Backup, file string, partial bool) error {
	if !filepath.IsAbs(file) {
		// Never let a file name be read as an option.
		return fmt.Errorf("the bundle path must be absolute: %s", file)
	}
	if partial {
		if err := checkPartialBundle(repoPath, backup); err != nil {
			return err
		}
	}
	args := append([]string{"bundle", "create", "--quiet", file}, backupRefs(backup)...)
	if partial {
		args = append(args, "--not", "--remotes")
	}
	output, err := gitCommand(repoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git bundle create failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// bundleFileName suggests a file name such as
// "myrepo-main-2026-09-25-1403.bundle", using the backup's local time, with
// "-partial" before the extension for a partial bundle.
func bundleFileName(repoPath string, backup gitpkg.Backup, partial bool) string {
	branch := "branches"
	if len(backup.Branches) == 1 {
		branch = strings.ReplaceAll(backup.Branches[0].Name, "/", "-")
	}
	created := backup.Created.Local().Format("2006-01-02-1504")
	suffix := ""
	if partial {
		suffix = "-partial"
	}
	return fmt.Sprintf("%s-%s-%s%s.bundle", filepath.Base(repoPath), branch, created, suffix)
}

// pruneAutoBackups keeps the newest automatic backups of each branch in
// saved, as many as Settings.AutoBackupsKept, after saved was made. Failing to
// prune only leaves extra backups behind, so errors are ignored.
func (app *App) pruneAutoBackups(repo *gogit.Repository, saved gitpkg.Backup) {
	app.settingsMutex.Lock()
	keep := app.loadSettings().AutoBackupsKept
	app.settingsMutex.Unlock()

	for _, branch := range saved.Branches {
		_, _ = gitpkg.PruneBackups(repo, branch.Name, keep)
	}
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
