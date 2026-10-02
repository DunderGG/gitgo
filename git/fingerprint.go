package git

import (
	"fmt"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// HasChanged reports whether the repository on disk no longer matches state,
// i.e. whether reopening it would give a different RepoState. GitGo's own
// edits reopen the state afterwards, so they do not count.
func HasChanged(state *RepoState) bool {
	return fingerprint(state.Repo, state.Path, state.Branch) != state.fingerprint
}

// fingerprint summarises the parts of the repository that a RepoState is
// computed from: what HEAD points at, the tip of branch, the remotes and the
// branch's upstream setting, every remote-tracking ref, the last fetch time
// and whether a merge or rebase is in progress. Equal fingerprints mean a
// RepoState opened now would show the same thing.
//
// It only reads refs and small files, so it is cheap enough to call every
// few seconds. Problems reading a part (a deleted branch, a broken config)
// are written into the fingerprint rather than returned, since they are a
// change worth reloading for too.
func fingerprint(repo *gogit.Repository, rootPath string, branch string) string {
	var parts []string
	add := func(name string, value any) {
		parts = append(parts, fmt.Sprintf("%s=%v", name, value))
	}

	// HEAD unresolved, so switching branches shows up even when both branches
	// point at the same commit.
	if head, err := repo.Storer.Reference(plumbing.HEAD); err != nil {
		add("HEAD", err)
	} else {
		add("HEAD", head.Strings())
	}

	if tip, err := repo.Storer.Reference(plumbing.NewBranchReferenceName(branch)); err != nil {
		add("branch", err)
	} else {
		add("branch", tip.Hash())
	}

	if cfg, err := repo.Config(); err != nil {
		add("upstream", err)
	} else {
		add("remotes", len(cfg.Remotes))
		if branchCfg, ok := cfg.Branches[branch]; ok {
			add("upstream", branchCfg.Remote+" "+branchCfg.Merge.String())
		}
	}

	var remoteRefs []string
	if refs, err := repo.References(); err != nil {
		add("refs", err)
	} else {
		_ = refs.ForEach(func(ref *plumbing.Reference) error {
			if ref.Name().IsRemote() && ref.Type() == plumbing.HashReference {
				remoteRefs = append(remoteRefs, ref.Name().String()+" "+ref.Hash().String())
			}
			return nil
		})
		refs.Close()
	}
	sort.Strings(remoteRefs)
	add("remote-refs", strings.Join(remoteRefs, ","))

	add("fetched", lastFetchTime(repo).UnixNano())
	add("in-progress", detectInProgressOperation(rootPath) != nil)

	return strings.Join(parts, "\n")
}
