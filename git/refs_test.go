package git_test

import (
	"reflect"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// setupRefsRepo creates main with commits first, second and third (checked
// out), plus refs around the "second" commit:
//
//	at-second      branch at second            → moved with the edit
//	at-third       branch at third             → moved with the edit
//	fork           branch with a commit on top of second → forked
//	at-first       branch at first (below the edit)      → unaffected
//	v-second       lightweight tag at second   → tag warning
//	v-third        annotated tag at third      → tag warning
//	v-first        tag at first                → unaffected
func setupRefsRepo(test *testing.T) (string, func(...string)) {
	test.Helper()
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)
	addCommit(test, dir, "third", gitCmd)

	gitCmd("branch", "at-second", "HEAD~1")
	gitCmd("branch", "at-third", "HEAD")
	gitCmd("branch", "at-first", "HEAD~2")
	gitCmd("tag", "v-second", "HEAD~1")
	gitCmd("tag", "-a", "v-third", "-m", "release", "HEAD")
	gitCmd("tag", "v-first", "HEAD~2")

	gitCmd("switch", "-c", "fork", "HEAD~1")
	addCommit(test, dir, "fork-only", gitCmd)
	gitCmd("switch", "main")
	return dir, gitCmd
}

// TestFindAffectedRefs_ListsBranchesForksAndTags verifies which refs are
// reported for an edit of the "second" commit on main.
func TestFindAffectedRefs_ListsBranchesForksAndTags(test *testing.T) {
	dir, _ := setupRefsRepo(test)
	target := revParse(test, dir, "main~1")

	got, err := git.FindAffectedRefs(mustOpen(test, dir), target)
	if err != nil {
		test.Fatalf("FindAffectedRefs: %v", err)
	}

	want := []git.AffectedRef{
		{Name: "at-second", Kind: git.AffectedBranch},
		{Name: "at-third", Kind: git.AffectedBranch},
		{Name: "fork", Kind: git.AffectedForkedBranch},
		{Name: "v-second", Kind: git.AffectedTag},
		{Name: "v-third", Kind: git.AffectedTag},
	}
	if !reflect.DeepEqual(got, want) {
		test.Errorf("FindAffectedRefs = %+v, want %+v", got, want)
	}
}

// TestFindAffectedRefs_NoneForLoneBranch verifies that an edit with no other
// refs around reports nothing.
func TestFindAffectedRefs_NoneForLoneBranch(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)
	addCommit(test, dir, "second", gitCmd)

	got, err := git.FindAffectedRefs(mustOpen(test, dir), revParse(test, dir, "HEAD~1"))
	if err != nil {
		test.Fatalf("FindAffectedRefs: %v", err)
	}
	if len(got) != 0 {
		test.Errorf("FindAffectedRefs = %+v, want none", got)
	}
}

// TestRebaseRewrite_MovesRequestedBranches verifies that branches in
// MoveBranches follow their rewritten commits, while branches that were not
// requested, or do not point at a rewritten commit, stay where they were.
func TestRebaseRewrite_MovesRequestedBranches(test *testing.T) {
	dir, _ := setupRefsRepo(test)
	target := revParse(test, dir, "main~1")
	atThirdBefore := revParse(test, dir, "at-third")
	atFirstBefore := revParse(test, dir, "at-first")
	forkBefore := revParse(test, dir, "fork")

	opts := baseAmendOpts()
	opts.Message = "edited second\n"
	opts.MoveBranches = []string{"at-second", "at-first", "fork"}
	mustRebaseRewrite(test, mustOpen(test, dir), target, opts)

	if got, want := revParse(test, dir, "at-second"), revParse(test, dir, "main~1"); got != want {
		test.Errorf("at-second = %s, want new main~1 %s", got, want)
	}
	if got := reflogSubject(test, dir, "at-second"); got != "gitgo: edit commit "+target.String()[:7] {
		test.Errorf("at-second reflog message = %q", got)
	}

	unchanged := map[string]plumbing.Hash{"at-third": atThirdBefore, "at-first": atFirstBefore, "fork": forkBefore}
	for name, before := range unchanged {
		if got := revParse(test, dir, name); got != before {
			test.Errorf("%s moved to %s, want it unchanged at %s", name, got, before)
		}
	}
}

// TestAmendCommit_MovesRequestedBranch verifies that a branch pointing at the
// amended tip moves to the amended commit.
func TestAmendCommit_MovesRequestedBranch(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "original commit", gitCmd)
	gitCmd("branch", "same-tip")

	opts := baseAmendOpts()
	opts.Message = "amended\n"
	opts.MoveBranches = []string{"same-tip"}
	mustAmend(test, mustOpen(test, dir), opts)

	if got, want := revParse(test, dir, "same-tip"), revParse(test, dir, "main"); got != want {
		test.Errorf("same-tip = %s, want %s", got, want)
	}
}
