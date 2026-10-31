package git_test

import (
	"errors"
	"strings"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// mustPlanPush calls PlanPush and fails the test on error.
func mustPlanPush(test *testing.T, repoState *git.RepoState, hashes ...plumbing.Hash) git.PushPlan {
	test.Helper()
	plan, err := git.PlanPush(repoState, hashes)
	if err != nil {
		test.Fatalf("PlanPush: %v", err)
	}
	return plan
}

// assertPlanCommits checks that plan.Commits are exactly want, in order.
func assertPlanCommits(test *testing.T, plan git.PushPlan, want ...plumbing.Hash) {
	test.Helper()
	got := make([]plumbing.Hash, len(plan.Commits))
	for i, entry := range plan.Commits {
		got[i] = entry.Hash
	}
	if len(got) != len(want) {
		test.Fatalf("Commits = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			test.Fatalf("Commits = %v, want %v", got, want)
		}
	}
}

// runPushCommand runs the plan's command the way the user would in a
// terminal.
func runPushCommand(plan git.PushPlan, gitCmd func(...string)) {
	gitCmd(strings.Fields(plan.Command)[1:]...)
}

// TestPlanPush_PushesOldestCommitsOnly verifies the plan for a commit in the
// middle of the unpushed commits: it pushes that commit and the ones below
// it, and after running the command the newer commit is still unpushed.
func TestPlanPush_PushesOldestCommitsOnly(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	for _, message := range []string{"one", "two", "three"} {
		addCommit(test, localDir, message, gitCmd)
	}
	one := revParse(test, localDir, "HEAD~2")
	two := revParse(test, localDir, "HEAD~1")
	three := revParse(test, localDir, "HEAD")

	plan := mustPlanPush(test, mustOpen(test, localDir), two)

	if plan.Remote != "origin" || plan.Branch != "main" || plan.IsNewBranch {
		test.Errorf("Remote, Branch, IsNewBranch = %q, %q, %v, want origin, main, false", plan.Remote, plan.Branch, plan.IsNewBranch)
	}
	if plan.UpstreamRemote != "origin" || plan.UpstreamBranch != "main" {
		test.Errorf("upstream = %s/%s, want origin/main", plan.UpstreamRemote, plan.UpstreamBranch)
	}
	if want := "git push origin " + two.String() + ":main"; plan.Command != want {
		test.Errorf("Command = %q, want %q", plan.Command, want)
	}
	if plan.Tip != two || plan.LeftOut != 1 {
		test.Errorf("Tip, LeftOut = %s, %d, want %s, 1", plan.Tip, plan.LeftOut, two)
	}
	assertPlanCommits(test, plan, two, one)

	runPushCommand(plan, gitCmd)
	assertUnpushed(test, mustOpen(test, localDir), three)
}

// TestPlanPush_TakesCommitsBetweenSelected verifies that selected commits
// with a gap between them are pushed together with the commit in the gap,
// since git cannot push a commit without the ones below it.
func TestPlanPush_TakesCommitsBetweenSelected(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	for _, message := range []string{"one", "two", "three"} {
		addCommit(test, localDir, message, gitCmd)
	}
	one := revParse(test, localDir, "HEAD~2")
	two := revParse(test, localDir, "HEAD~1")
	three := revParse(test, localDir, "HEAD")

	// The order given does not matter: the newest commit is the tip.
	plan := mustPlanPush(test, mustOpen(test, localDir), one, three)

	if plan.Tip != three || plan.LeftOut != 0 {
		test.Errorf("Tip, LeftOut = %s, %d, want %s, 0", plan.Tip, plan.LeftOut, three)
	}
	assertPlanCommits(test, plan, three, two, one)
}

// TestPlanPush_RejectsPushedCommit verifies that a pushed commit cannot be
// planned.
func TestPlanPush_RejectsPushedCommit(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	pushed := revParse(test, localDir, "HEAD")
	addCommit(test, localDir, "one", gitCmd)

	_, err := git.PlanPush(mustOpen(test, localDir), []plumbing.Hash{revParse(test, localDir, "HEAD"), pushed})
	if !errors.Is(err, git.ErrAlreadyPushed) {
		test.Fatalf("err = %v, want ErrAlreadyPushed", err)
	}
}

// TestPlanPush_RejectsRepoWithoutRemote verifies that nothing is planned
// without a remote to push to.
func TestPlanPush_RejectsRepoWithoutRemote(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "one", gitCmd)

	_, err := git.PlanPush(mustOpen(test, dir), []plumbing.Hash{revParse(test, dir, "HEAD")})
	if !errors.Is(err, git.ErrNoRemote) {
		test.Fatalf("err = %v, want ErrNoRemote", err)
	}
}

// TestPlanPush_NewBranch verifies that a branch the remote does not have yet
// is pushed with its full ref name (git refuses a bare name after a commit
// hash there), and that once pushed, the pushed commits count as pushed even
// though the branch still has no upstream.
func TestPlanPush_NewBranch(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	gitCmd("switch", "-c", "feature")
	addCommit(test, localDir, "one", gitCmd)
	addCommit(test, localDir, "two", gitCmd)
	one := revParse(test, localDir, "HEAD~1")
	two := revParse(test, localDir, "HEAD")

	plan := mustPlanPush(test, mustOpen(test, localDir), one)

	if !plan.IsNewBranch || plan.Branch != "feature" || plan.UpstreamBranch != "" {
		test.Errorf("IsNewBranch, Branch, UpstreamBranch = %v, %q, %q, want true, feature, empty", plan.IsNewBranch, plan.Branch, plan.UpstreamBranch)
	}
	if want := "git push origin " + one.String() + ":refs/heads/feature"; plan.Command != want {
		test.Errorf("Command = %q, want %q", plan.Command, want)
	}

	runPushCommand(plan, gitCmd)
	assertUnpushed(test, mustOpen(test, localDir), two)
}

// TestPlanPush_UpstreamWithOtherName verifies that a branch tracking a branch
// with another name, as after `git switch -c feature origin/main`, is pushed
// to a branch of its own name rather than into the upstream.
func TestPlanPush_UpstreamWithOtherName(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	gitCmd("switch", "-c", "feature", "--track", "origin/main")
	addCommit(test, localDir, "one", gitCmd)

	plan := mustPlanPush(test, mustOpen(test, localDir), revParse(test, localDir, "HEAD"))

	if plan.Remote != "origin" || plan.Branch != "feature" || !plan.IsNewBranch {
		test.Errorf("Remote, Branch, IsNewBranch = %q, %q, %v, want origin, feature, true", plan.Remote, plan.Branch, plan.IsNewBranch)
	}
	if plan.UpstreamRemote != "origin" || plan.UpstreamBranch != "main" {
		test.Errorf("upstream = %s/%s, want origin/main", plan.UpstreamRemote, plan.UpstreamBranch)
	}
}

// TestPlanPush_UsesPushRemote verifies that branch.<name>.pushRemote picks
// the remote, as it does for git push.
func TestPlanPush_UsesPushRemote(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	gitCmd("remote", "add", "fork", makeRemote(test))
	gitCmd("config", "branch.main.pushRemote", "fork")
	addCommit(test, localDir, "one", gitCmd)
	one := revParse(test, localDir, "HEAD")

	plan := mustPlanPush(test, mustOpen(test, localDir), one)

	if want := "git push fork " + one.String() + ":refs/heads/main"; plan.Command != want {
		test.Errorf("Command = %q, want %q", plan.Command, want)
	}
}

// TestPlanPush_SeparateLinesOfHistory verifies that commits on both sides of
// a merge cannot be pushed without the merge, and that the merge takes both
// sides.
func TestPlanPush_SeparateLinesOfHistory(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	gitCmd("switch", "-c", "side")
	addCommit(test, localDir, "side-one", gitCmd)
	side := revParse(test, localDir, "HEAD")
	gitCmd("switch", "main")
	addCommit(test, localDir, "main-one", gitCmd)
	mainOne := revParse(test, localDir, "HEAD")
	gitCmd("merge", "--no-edit", "side")
	merge := revParse(test, localDir, "HEAD")
	repoState := mustOpen(test, localDir)

	if _, err := git.PlanPush(repoState, []plumbing.Hash{mainOne, side}); !errors.Is(err, git.ErrNoPushTip) {
		test.Fatalf("both sides: err = %v, want ErrNoPushTip", err)
	}

	plan := mustPlanPush(test, repoState, side, merge)
	assertPlanCommits(test, plan, merge, mainOne, side)
}

// TestPlanPush_RejectsNameAShellWouldRun verifies that no command is built
// for a branch name a shell would act on.
func TestPlanPush_RejectsNameAShellWouldRun(test *testing.T) {
	localDir, gitCmd := setupPushedRepo(test, "first")
	branch := "feat$(id)"
	gitCmd("switch", "-c", branch)
	addCommit(test, localDir, "one", gitCmd)

	_, err := git.PlanPush(mustOpenBranch(test, localDir, branch), []plumbing.Hash{revParse(test, localDir, "HEAD")})
	if !errors.Is(err, git.ErrUnsafePushName) {
		test.Fatalf("err = %v, want ErrUnsafePushName", err)
	}
}
