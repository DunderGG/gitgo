package git_test

import (
	"testing"

	"gitgo/git"
)

func TestParseWebHost(test *testing.T) {
	cases := []struct {
		remote   string
		provider string
		base     string
	}{
		{"https://github.com/owner/repo.git", "GitHub", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo", "GitHub", "https://github.com/owner/repo"},
		{"https://user:token@github.com/owner/repo.git/", "GitHub", "https://github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "GitHub", "https://github.com/owner/repo"},
		{"git@github.com-work:owner/repo.git", "GitHub", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "GitHub", "https://github.com/owner/repo"},
		{"git://github.com/owner/repo.git", "GitHub", "https://github.com/owner/repo"},
		{"https://github.example.com/owner/repo.git", "GitHub", "https://github.example.com/owner/repo"},
		{"https://GitHub.com/Owner/Repo.git", "GitHub", "https://github.com/Owner/Repo"},
		{"git@gitlab.com:group/subgroup/repo.git", "GitLab", "https://gitlab.com/group/subgroup/repo"},
		{"ssh://git@gitlab.example.com:2222/group/repo.git", "GitLab", "https://gitlab.example.com/group/repo"},
		{"http://gitlab.internal/group/repo.git", "GitLab", "http://gitlab.internal/group/repo"},
		{"https://user@bitbucket.org/team/repo.git", "Bitbucket", "https://bitbucket.org/team/repo"},
		{"git@bitbucket.org:team/repo.git", "Bitbucket", "https://bitbucket.org/team/repo"},
		// Bitbucket Server, unknown hosts and local paths are not supported.
		{"ssh://git@bitbucket.example.com:7999/proj/repo.git", "", ""},
		{"https://example.com/owner/repo.git", "", ""},
		{"git@github.com:repo.git", "", ""},
		{"/home/me/repos/repo.git", "", ""},
		{`C:\repos\repo.git`, "", ""},
		{"C:/repos/repo.git", "", ""},
		{"file:///home/me/github/owner/repo.git", "", ""},
		{"../github.com:owner/repo", "", ""},
		{"", "", ""},
	}
	for _, testCase := range cases {
		got := git.ParseWebHost(testCase.remote)
		if got.Provider != testCase.provider || got.BaseURL != testCase.base {
			test.Errorf("ParseWebHost(%q) = %+v, want {%s %s}", testCase.remote, got, testCase.provider, testCase.base)
		}
	}
}

func TestWebHost_CommitURL(test *testing.T) {
	hash := "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		host git.WebHost
		want string
	}{
		{git.WebHost{Provider: "GitHub", BaseURL: "https://github.com/o/r"}, "https://github.com/o/r/commit/" + hash},
		{git.WebHost{Provider: "GitLab", BaseURL: "https://gitlab.com/g/r"}, "https://gitlab.com/g/r/-/commit/" + hash},
		{git.WebHost{Provider: "Bitbucket", BaseURL: "https://bitbucket.org/t/r"}, "https://bitbucket.org/t/r/commits/" + hash},
		{git.WebHost{}, ""},
	}
	for _, testCase := range cases {
		if got := testCase.host.CommitURL(hash); got != testCase.want {
			test.Errorf("%+v.CommitURL = %q, want %q", testCase.host, got, testCase.want)
		}
	}
}

// TestOpen_WebHostFollowsTrackedRemote verifies which remote the web page is
// taken from: the one the branch tracks, else "origin", else the first by name.
func TestOpen_WebHostFollowsTrackedRemote(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "first", gitCmd)

	if web := mustOpen(test, dir).Web; web != (git.WebHost{}) {
		test.Fatalf("Web without remotes = %+v, want the zero value", web)
	}

	gitCmd("remote", "add", "upstream", "git@gitlab.com:group/repo.git")
	gitCmd("remote", "add", "fork", "git@bitbucket.org:me/repo.git")
	if got := mustOpen(test, dir).Web.Provider; got != "Bitbucket" {
		test.Errorf("Web.Provider with no origin = %q, want the first remote by name (Bitbucket)", got)
	}

	gitCmd("remote", "add", "origin", "https://github.com/me/repo.git")
	if got := mustOpen(test, dir).Web.Provider; got != "GitHub" {
		test.Errorf("Web.Provider with origin = %q, want GitHub", got)
	}

	gitCmd("config", "branch.main.remote", "upstream")
	gitCmd("config", "branch.main.merge", "refs/heads/main")
	if got := mustOpen(test, dir).Web; got.BaseURL != "https://gitlab.com/group/repo" {
		test.Errorf("Web tracking upstream = %+v, want gitlab.com/group/repo", got)
	}
}
