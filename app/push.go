package app

import (
	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// PlanPush works out the git command that pushes the given unpushed commits,
// by full hash, and leaves the newer ones unpushed, for the user to run in a
// terminal: GitGo never pushes. A commit cannot be pushed without the ones
// below it, so the plan lists every commit the command pushes. See
// git.PlanPush.
// OpenRepository must be called before this method.
func (app *App) PlanPush(hashes []string) (PushPlan, error) {
	state, err := app.openState()
	if err != nil {
		return PushPlan{}, err
	}
	if err := checkHashes(hashes); err != nil {
		return PushPlan{}, err
	}

	// Read the repository again, so a commit pushed since the list was loaded
	// is refused. Unlike an edit, the fresh state is not kept: the list still
	// shows the old one, and HasExternalChanges goes on reporting the change.
	fresh, err := gitpkg.OpenBranch(state.Path, state.Branch)
	if err != nil {
		return PushPlan{}, err
	}

	targets := make([]plumbing.Hash, len(hashes))
	for i, hash := range hashes {
		targets[i] = plumbing.NewHash(hash)
	}
	plan, err := gitpkg.PlanPush(fresh, targets)
	if err != nil {
		return PushPlan{}, err
	}
	return PushPlan{
		Command:        plan.Command,
		Remote:         plan.Remote,
		Branch:         plan.Branch,
		IsNewBranch:    plan.IsNewBranch,
		UpstreamRemote: plan.UpstreamRemote,
		UpstreamBranch: plan.UpstreamBranch,
		Commits:        commitSummariesFromEntries(plan.Commits),
		LeftOut:        plan.LeftOut,
	}, nil
}
