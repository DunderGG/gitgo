package git_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// mustBackup calls CreateBackup and fails the test on error.
func mustBackup(test *testing.T, repoState *git.RepoState, kind git.BackupKind, others ...string) git.Backup {
	test.Helper()
	backup, err := git.CreateBackup(repoState, kind, others)
	if err != nil {
		test.Fatalf("git.CreateBackup: %v", err)
	}
	return backup
}

// mustListBackups calls ListBackups and fails the test on error.
func mustListBackups(test *testing.T, dir string) []git.Backup {
	test.Helper()
	backups, err := git.ListBackups(mustOpen(test, dir).Repo)
	if err != nil {
		test.Fatalf("git.ListBackups: %v", err)
	}
	return backups
}

// mustPlanRestore calls PlanRestore and fails the test on error.
func mustPlanRestore(test *testing.T, dir, id string) git.RestorePlan {
	test.Helper()
	plan, err := git.PlanRestore(dir, id)
	if err != nil {
		test.Fatalf("git.PlanRestore: %v", err)
	}
	return plan
}

// expectedTips returns the current tips from a plan, as the UI passes them
// back to RestoreBackup.
func expectedTips(plan git.RestorePlan) map[string]plumbing.Hash {
	tips := make(map[string]plumbing.Hash)
	for _, branch := range plan.Branches {
		tips[branch.Branch] = branch.CurrentTip
	}
	return tips
}

// amendMessage amends the checked-out branch's tip with a new message.
func amendMessage(test *testing.T, dir, message string, moveBranches ...string) {
	test.Helper()
	opts := baseAmendOpts()
	opts.Message = message + "\n"
	opts.MoveBranches = moveBranches
	mustAmend(test, mustOpen(test, dir), opts)
}

// TestCreateBackup_SavesBranchTip verifies that a backup is a ref under
// refs/gitgo/backups/ pointing at the branch tip, and is listed.
func TestCreateBackup_SavesBranchTip(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	tip := revParse(test, dir, "main")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual)

	if backup.Kind != git.BackupManual {
		test.Errorf("Kind = %q, want manual", backup.Kind)
	}
	if hash, ok := backup.Tip("main"); !ok || hash != tip {
		test.Errorf("Tip(main) = %s, %v, want %s", hash, ok, tip)
	}
	refs := gitOutputFromDir(test, dir, "git", "for-each-ref", "--format=%(refname) %(objectname)", "refs/gitgo/backups")
	want := "refs/gitgo/backups/" + backup.ID + "/main " + tip.String()
	if refs != want {
		test.Errorf("for-each-ref = %q, want %q", refs, want)
	}

	backups := mustListBackups(test, dir)
	if len(backups) != 1 || backups[0].ID != backup.ID {
		test.Fatalf("ListBackups = %+v, want the new backup", backups)
	}
	if !backups[0].Created.Equal(backup.Created) {
		test.Errorf("Created = %v, want %v", backups[0].Created, backup.Created)
	}
}

// TestCreateBackup_IncludesOtherBranches verifies that the named branches are
// saved alongside the edited one, and that missing ones are skipped.
func TestCreateBackup_IncludesOtherBranches(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	gitCmd("branch", "feature/x")
	addCommit(test, dir, "second", gitCmd)

	backup := mustBackup(test, mustOpen(test, dir), git.BackupAuto, "feature/x", "gone", "main")

	if len(backup.Branches) != 2 {
		test.Fatalf("Branches = %+v, want main and feature/x", backup.Branches)
	}
	if hash, _ := backup.Tip("feature/x"); hash != revParse(test, dir, "feature/x") {
		test.Errorf("Tip(feature/x) = %s, want %s", hash, revParse(test, dir, "feature/x"))
	}
	listed := mustListBackups(test, dir)
	if len(listed) != 1 || len(listed[0].Branches) != 2 || listed[0].Branches[0].Name != "feature/x" {
		test.Errorf("ListBackups = %+v, want one backup with feature/x and main", listed)
	}
}

// TestCreateBackup_KeepsCommitsThroughGC verifies that the backed-up commits
// survive an edit followed by an aggressive gc that expires the reflog.
func TestCreateBackup_KeepsCommitsThroughGC(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	original := revParse(test, dir, "main")

	mustBackup(test, mustOpen(test, dir), git.BackupManual)
	amendMessage(test, dir, "amended")

	gitCmd("reflog", "expire", "--expire=now", "--all")
	gitCmd("gc", "--prune=now")

	cmd := exec.Command("git", "cat-file", "-e", original.String())
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		test.Errorf("backed-up commit %s was pruned: %v", original, err)
	}
	// gc packs the refs; the backup must still be listed and restorable.
	backups := mustListBackups(test, dir)
	if len(backups) != 1 {
		test.Fatalf("ListBackups after gc = %+v, want one", backups)
	}
	plan := mustPlanRestore(test, dir, backups[0].ID)
	if _, err := git.RestoreBackup(dir, backups[0].ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	if got := revParse(test, dir, "main"); got != original {
		test.Errorf("main = %s, want %s", got, original)
	}
}

// TestCreateBackup_NotPushed verifies that `git push --all` leaves backups
// out.
func TestCreateBackup_NotPushed(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	remoteDir := makeRemote(test)
	gitCmd("remote", "add", "origin", remoteDir)

	mustBackup(test, mustOpen(test, dir), git.BackupManual)
	gitCmd("push", "--all", "origin")

	if refs := gitOutputFromDir(test, remoteDir, "git", "for-each-ref", "refs/gitgo"); refs != "" {
		test.Errorf("remote has backup refs: %q", refs)
	}
}

// TestListBackups_NewestFirst verifies the order and that backups made in
// quick succession get distinct IDs.
func TestListBackups_NewestFirst(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	var made []git.Backup
	for range 3 {
		made = append(made, mustBackup(test, mustOpen(test, dir), git.BackupAuto))
	}

	listed := mustListBackups(test, dir)
	if len(listed) != 3 {
		test.Fatalf("ListBackups returned %d backups, want 3", len(listed))
	}
	for i := range listed {
		if listed[i].ID != made[2-i].ID {
			test.Errorf("listed[%d] = %s, want %s", i, listed[i].ID, made[2-i].ID)
		}
	}
}

// TestListBackups_IgnoresForeignRefs verifies that refs under the backup
// namespace with an unknown layout are not listed.
func TestListBackups_IgnoresForeignRefs(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("update-ref", "refs/gitgo/backups/not-a-backup/main", "HEAD")
	gitCmd("update-ref", "refs/gitgo/backups/loose", "HEAD")

	if backups := mustListBackups(test, dir); len(backups) != 0 {
		test.Errorf("ListBackups = %+v, want none", backups)
	}
}

// TestDeleteBackup_RemovesRefs verifies that every ref of the backup goes and
// other backups stay.
func TestDeleteBackup_RemovesRefs(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")

	doomed := mustBackup(test, mustOpen(test, dir), git.BackupManual, "feature")
	kept := mustBackup(test, mustOpen(test, dir), git.BackupManual)

	if err := git.DeleteBackup(mustOpen(test, dir).Repo, doomed.ID); err != nil {
		test.Fatalf("git.DeleteBackup: %v", err)
	}
	backups := mustListBackups(test, dir)
	if len(backups) != 1 || backups[0].ID != kept.ID {
		test.Errorf("ListBackups = %+v, want only %s", backups, kept.ID)
	}

	err := git.DeleteBackup(mustOpen(test, dir).Repo, doomed.ID)
	if !errors.Is(err, git.ErrBackupNotFound) {
		test.Errorf("second delete: expected ErrBackupNotFound, got %v", err)
	}
}

// TestPruneBackups_KeepsNewestAutomatic verifies that only the branch's older
// automatic backups lose that branch: manual backups and the other branches of
// a pruned backup stay.
func TestPruneBackups_KeepsNewestAutomatic(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")

	oldest := mustBackup(test, mustOpen(test, dir), git.BackupAuto, "feature")
	manual := mustBackup(test, mustOpen(test, dir), git.BackupManual)
	older := mustBackup(test, mustOpen(test, dir), git.BackupAuto)
	newest := mustBackup(test, mustOpen(test, dir), git.BackupAuto)

	removed, err := git.PruneBackups(mustOpen(test, dir).Repo, "main", 1)
	if err != nil {
		test.Fatalf("git.PruneBackups: %v", err)
	}
	if removed != 2 {
		test.Errorf("removed = %d, want 2", removed)
	}

	backups := mustListBackups(test, dir)
	var ids []string
	for _, backup := range backups {
		ids = append(ids, backup.ID)
	}
	want := []string{newest.ID, manual.ID, oldest.ID}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		test.Fatalf("remaining = %v, want %v (older %s pruned)", ids, want, older.ID)
	}
	if _, ok := backups[2].Tip("main"); ok {
		test.Errorf("oldest backup still holds main")
	}
	if _, ok := backups[2].Tip("feature"); !ok {
		test.Errorf("oldest backup lost feature")
	}
}

// TestRestoreBackup_UndoesEdit verifies that restoring moves the checked-out
// branch back after an edit, and writes a reflog entry.
func TestRestoreBackup_UndoesEdit(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	original := revParse(test, dir, "main")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual)
	amendMessage(test, dir, "first edit")
	amendMessage(test, dir, "second edit")
	edited := revParse(test, dir, "main")

	plan := mustPlanRestore(test, dir, backup.ID)
	if err := plan.Err(); err != nil {
		test.Fatalf("plan blocked: %v", err)
	}
	branch := plan.Branches[0]
	if branch.CurrentTip != edited || !branch.CheckedOut || branch.RemovedCount != 1 || branch.ReturnedCount != 1 {
		test.Errorf("plan = %+v", branch)
	}
	if len(branch.Removed) != 1 || branch.Removed[0].Subject != "second edit" {
		test.Errorf("Removed = %+v, want the second edit", branch.Removed)
	}
	if len(branch.Returned) != 1 || branch.Returned[0].Subject != "original commit" {
		test.Errorf("Returned = %+v, want the original commit", branch.Returned)
	}

	if _, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	if got := revParse(test, dir, "main"); got != original {
		test.Errorf("main = %s, want %s", got, original)
	}
	reflog := gitOutputFromDir(test, dir, "git", "reflog", "-1", "--format=%gs", "main")
	if reflog != "gitgo: restore backup "+backup.ID {
		test.Errorf("reflog = %q", reflog)
	}
	if status := gitOutputFromDir(test, dir, "git", "status", "--porcelain"); status != "" {
		test.Errorf("working tree not clean after restore: %q", status)
	}
	// The backup is kept, so it can be restored again later.
	if backups := mustListBackups(test, dir); len(backups) != 1 {
		test.Errorf("ListBackups = %+v, want the backup kept", backups)
	}
}

// TestRestoreBackup_RestoresSeveralBranches verifies that a backup made
// before an edit that moved another branch brings both back.
func TestRestoreBackup_RestoresSeveralBranches(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")
	original := revParse(test, dir, "main")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupAuto, "feature")
	amendMessage(test, dir, "edited", "feature")
	if revParse(test, dir, "feature") == original {
		test.Fatalf("the edit did not move feature")
	}

	plan := mustPlanRestore(test, dir, backup.ID)
	if _, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	for _, branch := range []string{"main", "feature"} {
		if got := revParse(test, dir, branch); got != original {
			test.Errorf("%s = %s, want %s", branch, got, original)
		}
	}
}

// TestRestoreBackup_SkipsDeletedBranch verifies that a branch deleted since
// the backup is reported as missing and not recreated.
func TestRestoreBackup_SkipsDeletedBranch(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")
	original := revParse(test, dir, "main")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupAuto, "feature")
	amendMessage(test, dir, "edited")
	gitCmd("branch", "-D", "feature")

	plan := mustPlanRestore(test, dir, backup.ID)
	var missing []string
	for _, branch := range plan.Branches {
		if branch.Missing {
			missing = append(missing, branch.Branch)
		}
	}
	if strings.Join(missing, ",") != "feature" {
		test.Errorf("missing = %v, want [feature]", missing)
	}
	if _, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	if got := revParse(test, dir, "main"); got != original {
		test.Errorf("main = %s, want %s", got, original)
	}
	if branches := gitOutputFromDir(test, dir, "git", "branch", "--list", "feature"); branches != "" {
		test.Errorf("feature was recreated")
	}
}

// TestRestoreBackup_RejectsPushed verifies that a restore that would remove
// pushed commits is refused and changes nothing.
func TestRestoreBackup_RejectsPushed(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	remoteDir := makeRemote(test)
	gitCmd("remote", "add", "origin", remoteDir)

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual)
	amendMessage(test, dir, "edited")
	gitCmd("push", "-u", "origin", "main")
	edited := revParse(test, dir, "main")

	plan := mustPlanRestore(test, dir, backup.ID)
	if !errors.Is(plan.Err(), git.ErrRestorePushed) {
		test.Errorf("plan.Err() = %v, want ErrRestorePushed", plan.Err())
	}
	_, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan))
	if !errors.Is(err, git.ErrRestorePushed) {
		test.Fatalf("expected ErrRestorePushed, got %v", err)
	}
	if got := revParse(test, dir, "main"); got != edited {
		test.Errorf("main = %s, want unchanged %s", got, edited)
	}
}

// TestRestoreBackup_RejectsFileChangesOnCheckedOutBranch verifies the
// tree-match rule: a commit with file changes made since the backup blocks a
// restore of the checked-out branch, but not of another branch.
func TestRestoreBackup_RejectsFileChangesOnCheckedOutBranch(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual, "feature")
	addCommit(test, dir, "new work on main", gitCmd)
	newest := revParse(test, dir, "main")

	plan := mustPlanRestore(test, dir, backup.ID)
	if !errors.Is(plan.Err(), git.ErrRestoreFilesChanged) {
		test.Errorf("plan.Err() = %v, want ErrRestoreFilesChanged", plan.Err())
	}
	_, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan))
	if !errors.Is(err, git.ErrRestoreFilesChanged) {
		test.Fatalf("expected ErrRestoreFilesChanged, got %v", err)
	}
	if got := revParse(test, dir, "main"); got != newest {
		test.Errorf("main = %s, want unchanged %s", got, newest)
	}

	// The same kind of change on a branch that is not checked out is fine:
	// only its ref moves.
	gitCmd("checkout", "-q", "feature")
	plan = mustPlanRestore(test, dir, backup.ID)
	for _, branch := range plan.Branches {
		if branch.Branch == "main" && branch.Blocked != nil {
			test.Errorf("main blocked while not checked out: %v", branch.Blocked)
		}
	}
	if _, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	if got, want := revParse(test, dir, "main"), revParse(test, dir, "feature"); got != want {
		test.Errorf("main = %s, want %s", got, want)
	}
}

// TestRestoreBackup_RejectsWhenBranchMoved verifies the compare-and-swap: a
// branch that moved after the plan was shown is left alone.
func TestRestoreBackup_RejectsWhenBranchMoved(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual)
	amendMessage(test, dir, "first edit")
	plan := mustPlanRestore(test, dir, backup.ID)
	amendMessage(test, dir, "second edit")
	moved := revParse(test, dir, "main")

	_, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan))
	if !errors.Is(err, git.ErrBranchChanged) {
		test.Fatalf("expected ErrBranchChanged, got %v", err)
	}
	if got := revParse(test, dir, "main"); got != moved {
		test.Errorf("main = %s, want unchanged %s", got, moved)
	}
}

// TestPlanRestore_UnknownBackup verifies the error for a deleted backup.
func TestPlanRestore_UnknownBackup(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)

	_, err := git.PlanRestore(dir, "2026-01-01T00-00-00.000Z-manual")
	if !errors.Is(err, git.ErrBackupNotFound) {
		test.Errorf("expected ErrBackupNotFound, got %v", err)
	}
}

// TestSetBackupName_NamesAndClears verifies that a name is listed, that the
// backup still restores and survives gc, and that an empty name removes it.
func TestSetBackupName_NamesAndClears(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "feature")
	original := revParse(test, dir, "main")

	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual, "feature")
	if err := git.SetBackupName(mustOpen(test, dir), backup.ID, "  before date spread  "); err != nil {
		test.Fatalf("git.SetBackupName: %v", err)
	}

	listed := mustListBackups(test, dir)
	if len(listed) != 1 || listed[0].Name != "before date spread" {
		test.Fatalf("ListBackups = %+v, want the trimmed name", listed)
	}
	for _, branch := range listed[0].Branches {
		if branch.Hash != original {
			test.Errorf("%s = %s, want the peeled commit %s", branch.Name, branch.Hash, original)
		}
	}
	// git itself peels the ref to the commit.
	if got := revParse(test, dir, "refs/gitgo/backups/"+backup.ID+"/main^{commit}"); got != original {
		test.Errorf("rev-parse = %s, want %s", got, original)
	}

	amendMessage(test, dir, "edited", "feature")
	gitCmd("reflog", "expire", "--expire=now", "--all")
	gitCmd("gc", "--prune=now")
	plan := mustPlanRestore(test, dir, backup.ID)
	if _, err := git.RestoreBackup(dir, backup.ID, expectedTips(plan)); err != nil {
		test.Fatalf("git.RestoreBackup: %v", err)
	}
	if got := revParse(test, dir, "main"); got != original {
		test.Errorf("main = %s, want %s", got, original)
	}

	if err := git.SetBackupName(mustOpen(test, dir), backup.ID, ""); err != nil {
		test.Fatalf("clearing the name: %v", err)
	}
	listed = mustListBackups(test, dir)
	if listed[0].Name != "" {
		test.Errorf("Name = %q after clearing, want empty", listed[0].Name)
	}
	if kind := gitOutputFromDir(test, dir, "git", "cat-file", "-t", "refs/gitgo/backups/"+backup.ID+"/main"); kind != "commit" {
		test.Errorf("ref points at a %s after clearing, want a commit", kind)
	}
}

// TestSetBackupName_RejectsInvalidName verifies the length and single-line
// rules, and the error for an unknown backup.
func TestSetBackupName_RejectsInvalidName(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	backup := mustBackup(test, mustOpen(test, dir), git.BackupManual)

	for _, name := range []string{"two\nlines", strings.Repeat("é", 101)} {
		if err := git.SetBackupName(mustOpen(test, dir), backup.ID, name); !errors.Is(err, git.ErrInvalidBackupName) {
			test.Errorf("SetBackupName(%q) error = %v, want ErrInvalidBackupName", name, err)
		}
	}
	if err := git.SetBackupName(mustOpen(test, dir), backup.ID, strings.Repeat("é", 100)); err != nil {
		test.Errorf("100 characters rejected: %v", err)
	}
	err := git.SetBackupName(mustOpen(test, dir), "2026-01-01T00-00-00.000Z-manual", "x")
	if !errors.Is(err, git.ErrBackupNotFound) {
		test.Errorf("unknown backup error = %v, want ErrBackupNotFound", err)
	}
}
