package app

import (
	"errors"
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
