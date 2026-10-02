package app

import (
	"errors"
	"path/filepath"
	"testing"

	gitpkg "gitgo/git"
)

// currentTips returns each branch's current tip from a restore plan, as the
// frontend passes them back to RestoreBackup.
func currentTips(plan RestorePlanInfo) map[string]string {
	tips := make(map[string]string)
	for _, branch := range plan.Branches {
		tips[branch.Name] = branch.CurrentTip
	}
	return tips
}

// mustListBackups calls ListBackups and fails the test on error.
func mustListBackups(test *testing.T, app *App) []BackupInfo {
	test.Helper()
	backups, err := app.ListBackups()
	if err != nil {
		test.Fatalf("ListBackups: %v", err)
	}
	return backups
}

// TestCreateBackup_ListsBackup verifies that a manual backup of the open
// branch is listed with its tip.
func TestCreateBackup_ListsBackup(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	tip := runGit(test, dir, "rev-parse", "HEAD")

	backup, err := app.CreateBackup()
	if err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}
	if backup.Kind != "manual" || len(backup.Branches) != 1 {
		test.Fatalf("backup = %+v, want one manual branch", backup)
	}
	branch := backup.Branches[0]
	if branch.Name != "main" || branch.Hash != tip || branch.Subject != "local" || !branch.Current || !branch.Exists {
		test.Errorf("branch = %+v", branch)
	}

	backups := mustListBackups(test, app)
	if len(backups) != 1 || backups[0].ID != backup.ID {
		test.Errorf("ListBackups = %+v, want the new backup", backups)
	}
}

// TestRestoreBackup_RestoresAndSavesCurrentState verifies that a restore
// brings back the backed-up commit, saves the replaced state as an automatic
// backup, and can be undone.
func TestRestoreBackup_RestoresAndSavesCurrentState(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	original := runGit(test, dir, "rev-parse", "HEAD")

	backup, err := app.CreateBackup()
	if err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}
	if _, err := app.UpdateCommit(editRequest(original)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	edited := runGit(test, dir, "rev-parse", "HEAD")

	plan, err := app.PlanRestore(backup.ID)
	if err != nil {
		test.Fatalf("PlanRestore: %v", err)
	}
	if plan.Problem != "" || len(plan.Branches) != 1 || plan.Branches[0].RemovedCount != 1 {
		test.Fatalf("plan = %+v", plan)
	}

	result, err := app.RestoreBackup(backup.ID, currentTips(plan))
	if err != nil || !result.Success {
		test.Fatalf("RestoreBackup = %+v, %v", result, err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != original {
		test.Errorf("HEAD = %s, want %s", got, original)
	}

	backups := mustListBackups(test, app)
	if len(backups) != 2 || backups[0].Kind != "auto" || backups[0].Branches[0].Hash != edited {
		test.Errorf("ListBackups = %+v, want an automatic backup of %s first", backups, edited)
	}

	if !app.CanUndo() {
		test.Fatalf("CanUndo = false after a restore")
	}
	if _, err := app.UndoLastOperation(); err != nil {
		test.Fatalf("UndoLastOperation: %v", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != edited {
		test.Errorf("HEAD after undo = %s, want %s", got, edited)
	}
}

// TestRestoreBackup_RefusalLeavesNoBackup verifies that a refused restore
// changes nothing and does not save the current state.
func TestRestoreBackup_RefusalLeavesNoBackup(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	original := runGit(test, dir, "rev-parse", "HEAD")

	backup, err := app.CreateBackup()
	if err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}
	if _, err := app.UpdateCommit(editRequest(original)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	plan, err := app.PlanRestore(backup.ID)
	if err != nil {
		test.Fatalf("PlanRestore: %v", err)
	}
	runGit(test, dir, "push")
	pushed := runGit(test, dir, "rev-parse", "HEAD")

	_, err = app.RestoreBackup(backup.ID, currentTips(plan))
	if !errors.Is(err, gitpkg.ErrRestorePushed) {
		test.Fatalf("RestoreBackup error = %v, want ErrRestorePushed", err)
	}
	if got := runGit(test, dir, "rev-parse", "HEAD"); got != pushed {
		test.Errorf("HEAD = %s, want unchanged %s", got, pushed)
	}
	if backups := mustListBackups(test, app); len(backups) != 1 {
		test.Errorf("ListBackups = %+v, want only the manual backup", backups)
	}
}

// TestRestoreBackup_RejectsStalePlan verifies that a branch that moved after
// the plan was shown is not restored.
func TestRestoreBackup_RejectsStalePlan(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)

	backup, err := app.CreateBackup()
	if err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}
	commitFile(test, dir, "newer")
	plan, err := app.PlanRestore(backup.ID)
	if err != nil {
		test.Fatalf("PlanRestore: %v", err)
	}
	commitFile(test, dir, "newest")

	_, err = app.RestoreBackup(backup.ID, currentTips(plan))
	if !errors.Is(err, gitpkg.ErrBranchChanged) {
		test.Fatalf("RestoreBackup error = %v, want ErrBranchChanged", err)
	}
}

// TestDeleteBackup_RemovesBackup verifies DeleteBackup and its error for an
// unknown ID.
func TestDeleteBackup_RemovesBackup(test *testing.T) {
	_, app := setupRepoWithUnpushedCommit(test)

	backup, err := app.CreateBackup()
	if err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}
	if err := app.DeleteBackup(backup.ID); err != nil {
		test.Fatalf("DeleteBackup: %v", err)
	}
	if backups := mustListBackups(test, app); len(backups) != 0 {
		test.Errorf("ListBackups = %+v, want none", backups)
	}
	if err := app.DeleteBackup(backup.ID); !errors.Is(err, gitpkg.ErrBackupNotFound) {
		test.Errorf("second DeleteBackup error = %v, want ErrBackupNotFound", err)
	}
}

// TestUpdateCommit_BacksUpFirst verifies that an edit with Backup saves the
// edited branch and the branches it moves, and that a restore of that backup
// undoes the edit.
func TestUpdateCommit_BacksUpFirst(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	app.settingsPath = filepath.Join(test.TempDir(), "settings.json")
	local := runGit(test, dir, "rev-parse", "HEAD")
	runGit(test, dir, "branch", "same-tip")

	req := editRequest(local)
	req.MoveBranches = []string{"same-tip"}
	req.Backup = true
	if result, err := app.UpdateCommit(req); err != nil || !result.Success {
		test.Fatalf("UpdateCommit = %+v, %v", result, err)
	}

	backups := mustListBackups(test, app)
	if len(backups) != 1 || backups[0].Kind != "auto" || len(backups[0].Branches) != 2 {
		test.Fatalf("ListBackups = %+v, want one automatic backup of main and same-tip", backups)
	}
	for _, branch := range backups[0].Branches {
		if branch.Hash != local {
			test.Errorf("backup of %s = %s, want %s", branch.Name, branch.Hash, local)
		}
	}

	plan, err := app.PlanRestore(backups[0].ID)
	if err != nil {
		test.Fatalf("PlanRestore: %v", err)
	}
	if _, err := app.RestoreBackup(backups[0].ID, currentTips(plan)); err != nil {
		test.Fatalf("RestoreBackup: %v", err)
	}
	for _, branch := range []string{"main", "same-tip"} {
		if got := runGit(test, dir, "rev-parse", branch); got != local {
			test.Errorf("%s = %s after restore, want %s", branch, got, local)
		}
	}
}

// TestUpdateCommit_WithoutBackup verifies that no backup is made when the
// checkbox is off, nor when the edit is refused.
func TestUpdateCommit_WithoutBackup(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")

	if _, err := app.UpdateCommit(editRequest(local)); err != nil {
		test.Fatalf("UpdateCommit: %v", err)
	}
	if backups := mustListBackups(test, app); len(backups) != 0 {
		test.Fatalf("ListBackups = %+v after an edit without backup, want none", backups)
	}

	req := editRequest(runGit(test, dir, "rev-parse", "HEAD"))
	req.AuthorName = ""
	req.Backup = true
	if _, err := app.UpdateCommit(req); !errors.Is(err, gitpkg.ErrInvalidIdentity) {
		test.Fatalf("UpdateCommit error = %v, want ErrInvalidIdentity", err)
	}
	if backups := mustListBackups(test, app); len(backups) != 0 {
		test.Errorf("ListBackups = %+v after a refused edit, want none", backups)
	}
}

// TestEditCommits_KeepsAutoBackupsKept verifies that automatic backups beyond
// the setting are pruned, newest kept, and manual ones are left alone.
func TestEditCommits_KeepsAutoBackupsKept(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	app.settingsPath = filepath.Join(test.TempDir(), "settings.json")
	if err := app.SetBackupSettings(true, 2); err != nil {
		test.Fatalf("SetBackupSettings: %v", err)
	}
	if _, err := app.CreateBackup(); err != nil {
		test.Fatalf("CreateBackup: %v", err)
	}

	var tips []string
	for range 3 {
		tip := runGit(test, dir, "rev-parse", "HEAD")
		tips = append(tips, tip)
		req := BulkEditRequest{Hashes: []string{tip}, Minutes: 60, Backup: true}
		if result, err := app.EditCommits(req); err != nil || !result.Success {
			test.Fatalf("EditCommits = %+v, %v", result, err)
		}
	}

	var auto, manual int
	var kept []string
	for _, backup := range mustListBackups(test, app) {
		if backup.Kind == "manual" {
			manual++
			continue
		}
		auto++
		kept = append(kept, backup.Branches[0].Hash)
	}
	if manual != 1 || auto != 2 {
		test.Fatalf("manual = %d, auto = %d, want 1 and 2", manual, auto)
	}
	if kept[0] != tips[2] || kept[1] != tips[1] {
		test.Errorf("kept automatic backups of %v, want the newest two of %v", kept, tips)
	}
}
