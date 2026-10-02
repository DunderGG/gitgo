package app

import (
	"fmt"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// commitWebURL returns the commit's page on the hosting service of the
// repository in state. Only pushed commits are on the service, so an
// unpushed one is refused rather than opening a "not found" page.
func commitWebURL(state *gitpkg.RepoState, hash string) (string, error) {
	if err := checkHashes([]string{hash}); err != nil {
		return "", err
	}
	if state.Web.Provider == "" {
		return "", fmt.Errorf("the remote is not on GitHub, GitLab or Bitbucket")
	}
	if state.UnpushedHashes[plumbing.NewHash(hash)] {
		return "", fmt.Errorf("this commit has not been pushed, so it is not on %s yet", state.Web.Provider)
	}
	return state.Web.CommitURL(hash), nil
}

// OpenCommitOnWeb opens the page of a pushed commit, given by its full hash,
// on GitHub, GitLab or Bitbucket in the default browser. RepoInfo.WebHost
// tells the frontend whether the remote is on one of them.
// OpenRepository must be called before this method.
func (app *App) OpenCommitOnWeb(hash string) error {
	state, err := app.openState()
	if err != nil {
		return err
	}
	url, err := commitWebURL(state, hash)
	if err != nil {
		return err
	}
	runtime.BrowserOpenURL(app.ctx, url)
	return nil
}
