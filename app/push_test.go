package app

import (
	"errors"
	"testing"

	gitpkg "gitgo/git"
)

// TestPlanPush_ReturnsCommandAndCommits verifies the plan for the newest
// unpushed commit: the command and every commit it pushes.
func TestPlanPush_ReturnsCommandAndCommits(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	commitFile(test, dir, "newer")
	local := runGit(test, dir, "rev-parse", "HEAD~1")

	plan, err := app.PlanPush([]string{local})
	if err != nil {
		test.Fatalf("PlanPush: %v", err)
	}
	if want := "git push origin " + local + ":main"; plan.Command != want {
		test.Errorf("Command = %q, want %q", plan.Command, want)
	}
	if len(plan.Commits) != 1 || plan.Commits[0].Hash != local || !plan.Commits[0].IsUnpushed {
		test.Errorf("Commits = %+v, want only the unpushed %s", plan.Commits, local)
	}
	if plan.LeftOut != 1 {
		test.Errorf("LeftOut = %d, want 1", plan.LeftOut)
	}
}

// TestPlanPush_RefusesCommitPushedAfterOpen verifies that a commit pushed
// from a terminal after the repository was opened is refused, and that the
// change is still reported, since the list has not been reloaded.
func TestPlanPush_RefusesCommitPushedAfterOpen(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")
	runGit(test, dir, "push")

	if _, err := app.PlanPush([]string{local}); !errors.Is(err, gitpkg.ErrAlreadyPushed) {
		test.Fatalf("err = %v, want ErrAlreadyPushed", err)
	}
	if !app.HasExternalChanges() {
		test.Error("HasExternalChanges = false after the push, want true")
	}
}

func TestPlanPush_RejectsNonHash(test *testing.T) {
	_, app := setupRepoWithUnpushedCommit(test)
	if _, err := app.PlanPush([]string{"HEAD"}); err == nil {
		test.Fatal("PlanPush(HEAD): want an error")
	}
}
