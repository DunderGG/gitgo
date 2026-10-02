package app

import (
	"strings"
	"testing"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestCommitWebURL(test *testing.T) {
	pushed := "0123456789abcdef0123456789abcdef01234567"
	unpushed := "89abcdef0123456789abcdef0123456789abcdef"
	state := &gitpkg.RepoState{
		Web:            gitpkg.WebHost{Provider: "GitHub", BaseURL: "https://github.com/o/r"},
		UnpushedHashes: map[plumbing.Hash]bool{plumbing.NewHash(unpushed): true},
	}

	url, err := commitWebURL(state, pushed)
	if err != nil || url != "https://github.com/o/r/commit/"+pushed {
		test.Fatalf("pushed commit: url = %q, err = %v", url, err)
	}

	if _, err := commitWebURL(state, unpushed); err == nil || !strings.Contains(err.Error(), "not been pushed") {
		test.Errorf("unpushed commit: err = %v, want a not-pushed error", err)
	}
	if _, err := commitWebURL(state, "HEAD"); err == nil {
		test.Error("non-hash: want an error")
	}
	if _, err := commitWebURL(&gitpkg.RepoState{}, pushed); err == nil {
		test.Error("no web host: want an error")
	}
}

func TestOpenCommitOnWeb_RequiresOpenRepository(test *testing.T) {
	if err := New().OpenCommitOnWeb("0123456789abcdef0123456789abcdef01234567"); err == nil {
		test.Fatal("OpenCommitOnWeb without a repository: want an error")
	}
}
