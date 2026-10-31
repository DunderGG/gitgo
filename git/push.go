package git

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// PushPlan is how to push some of a branch's unpushed commits and leave the
// newer ones unpushed. Git pushes a commit together with every commit below
// it, so the plan pushes the newest chosen commit, and with it every
// unpushed commit it is built on.
type PushPlan struct {
	// Remote and Branch are where the push goes: a branch of the local
	// branch's own name, on the remote a plain git push would use.
	Remote string
	Branch string
	// IsNewBranch is true when there is no remote-tracking branch for Branch,
	// so as far as the last fetch knows, the push creates it.
	IsNewBranch bool
	// UpstreamRemote and UpstreamBranch are the branch the local branch
	// tracks, or empty without one. When the upstream has another name, the
	// push still goes to Branch, so the commits cannot land on, say, main
	// because a feature branch was started from origin/main.
	UpstreamRemote string
	UpstreamBranch string
	// Tip is the newest commit pushed.
	Tip plumbing.Hash
	// Commits are the commits the push takes, Tip first: Tip and every
	// unpushed commit below it.
	Commits []CommitEntry
	// LeftOut is how many unpushed commits the push does not take. They stay
	// unpushed and editable.
	LeftOut int
	// Command is the git command that does the push, for the user to run.
	Command string
}

// shellSafeName matches remote and branch names that every common shell
// (bash, zsh, PowerShell, cmd) passes on unchanged, so the command can be
// pasted as it is. Git allows names such as `$(...)` or `a&b`, which a shell
// would run, and a cloned repository's default branch name comes from the
// remote. A leading letter, digit or underscore also keeps a remote name
// from being read as an option.
var shellSafeName = regexp.MustCompile(`^[\p{L}\p{N}_][\p{L}\p{N}_./+-]*$`)

// PlanPush works out how to push the given unpushed commits of state.Branch
// without the commits above them. Nothing is pushed: the plan's Command is
// for the user to run in a terminal.
//
// A commit cannot be pushed without the commits below it, so when the given
// commits leave gaps, the plan takes those commits too; Commits lists every
// commit the push takes. PlanPush fails when there is no remote, when a
// commit has been pushed already, when no given commit has all the others
// below it, or when a name is not safe to paste into a shell.
func PlanPush(state *RepoState, hashes []plumbing.Hash) (PushPlan, error) {
	if len(hashes) == 0 {
		return PushPlan{}, errors.New("no commits selected")
	}
	plan, err := pushDestination(state)
	if err != nil {
		return PushPlan{}, err
	}
	for _, hash := range hashes {
		if !state.UnpushedHashes[hash] {
			return PushPlan{}, fmt.Errorf("%w: %s", ErrAlreadyPushed, hash.String()[:7])
		}
	}

	// The tip is the given commit that has all the others below it.
	for _, candidate := range hashes {
		commits, err := unpushedHistory(state, candidate)
		if err != nil {
			return PushPlan{}, err
		}
		if !containsAll(commits, hashes) {
			continue
		}

		destination := plan.Branch
		if plan.IsNewBranch {
			// Without a remote branch of that name, git cannot tell whether a
			// bare name after a commit hash means a branch or a tag.
			destination = "refs/heads/" + plan.Branch
		}
		plan.Tip = candidate
		plan.Commits = commits
		plan.LeftOut = len(state.UnpushedHashes) - len(commits)
		plan.Command = fmt.Sprintf("git push %s %s:%s", plan.Remote, candidate, destination)
		return plan, nil
	}
	return PushPlan{}, ErrNoPushTip
}

// pushDestination returns a PushPlan with only where the push goes filled in.
// The branch is the local branch's own name, as with git push's default
// push.default=simple, which refuses rather than push to an upstream with
// another name. The remote is the one git push picks from the repository's
// config: branch.<name>.pushRemote, remote.pushDefault, the remote the branch
// tracks, then origin, then the first by name.
func pushDestination(state *RepoState) (PushPlan, error) {
	cfg, err := state.Repo.Config()
	if err != nil {
		return PushPlan{}, fmt.Errorf("reading config: %w", err)
	}
	remote := pushRemote(cfg, state.Branch)
	if remote == "" {
		return PushPlan{}, ErrNoRemote
	}
	for _, name := range []struct{ kind, value string }{{"remote", remote}, {"branch", state.Branch}} {
		if !shellSafeName.MatchString(name.value) {
			return PushPlan{}, fmt.Errorf("%s %q: %w", name.kind, name.value, ErrUnsafePushName)
		}
	}

	plan := PushPlan{Remote: remote, Branch: state.Branch}
	_, err = state.Repo.Reference(plumbing.NewRemoteReferenceName(remote, state.Branch), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		plan.IsNewBranch = true
	} else if err != nil {
		return PushPlan{}, fmt.Errorf("reading %s/%s: %w", remote, state.Branch, err)
	}
	if branchCfg, ok := cfg.Branches[state.Branch]; ok && cfg.Remotes[branchCfg.Remote] != nil && branchCfg.Merge.IsBranch() {
		plan.UpstreamRemote = branchCfg.Remote
		plan.UpstreamBranch = branchCfg.Merge.Short()
	}
	return plan, nil
}

// pushRemote returns the remote a plain git push of the branch goes to (see
// pushDestination), or empty when there are no remotes. Only the
// repository's own config is read.
func pushRemote(cfg *config.Config, branchName string) string {
	for _, name := range []string{
		cfg.Raw.Section("branch").Subsection(branchName).Option("pushRemote"),
		cfg.Raw.Section("remote").Option("pushDefault"),
	} {
		if cfg.Remotes[name] != nil {
			return name
		}
	}
	return trackedRemote(cfg, branchName)
}

// unpushedHistory returns tip and every unpushed commit below it, depth
// first along the first parents, which for a straight history is newest
// first, as in the commit list. The walk stops at pushed commits, since
// every commit below a pushed one is pushed too.
func unpushedHistory(state *RepoState, tip plumbing.Hash) ([]CommitEntry, error) {
	var entries []CommitEntry
	seen := make(map[plumbing.Hash]bool)
	pending := []plumbing.Hash{tip}
	for len(pending) > 0 {
		hash := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[hash] || !state.UnpushedHashes[hash] {
			continue
		}
		seen[hash] = true

		commit, err := state.Repo.CommitObject(hash)
		if err != nil {
			return nil, fmt.Errorf("loading commit %s: %w", hash, err)
		}
		entries = append(entries, entryFromCommit(state, commit))
		// Last parent first, so the first parent is walked next.
		for i := len(commit.ParentHashes) - 1; i >= 0; i-- {
			pending = append(pending, commit.ParentHashes[i])
		}
	}
	return entries, nil
}

// containsAll reports whether every hash is among the entries.
func containsAll(entries []CommitEntry, hashes []plumbing.Hash) bool {
	found := make(map[plumbing.Hash]bool, len(entries))
	for _, entry := range entries {
		found[entry.Hash] = true
	}
	for _, hash := range hashes {
		if !found[hash] {
			return false
		}
	}
	return true
}
