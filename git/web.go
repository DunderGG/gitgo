package git

import (
	"net/url"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
)

// WebHost is the web page of the repository on a hosting service, worked out
// from a remote URL. The zero value means no supported service was found.
type WebHost struct {
	// Provider is the service's display name: "GitHub", "GitLab" or "Bitbucket".
	Provider string

	// BaseURL is the repository's page, for example
	// https://github.com/owner/repo, without a trailing slash.
	BaseURL string
}

// CommitURL returns the web page of the commit with the given full hash, or
// an empty string when host is the zero value.
func (host WebHost) CommitURL(hash string) string {
	switch host.Provider {
	case "GitHub":
		return host.BaseURL + "/commit/" + hash
	case "GitLab":
		return host.BaseURL + "/-/commit/" + hash
	case "Bitbucket":
		return host.BaseURL + "/commits/" + hash
	default:
		return ""
	}
}

// ParseWebHost works out the repository's web page from a remote URL in any
// of the forms git accepts: https://host/owner/repo.git,
// ssh://git@host:port/owner/repo.git or the scp-like git@host:owner/repo.git.
// GitHub (also GitHub Enterprise), GitLab (also self-managed, nested groups
// included) and bitbucket.org are recognised by the host name. Bitbucket
// Server is not, since its pages use another path layout. Local paths and
// unknown hosts give the zero WebHost.
func ParseWebHost(remoteURL string) WebHost {
	remoteURL = strings.TrimSpace(remoteURL)
	scheme := "https"
	var host, path string

	if strings.Contains(remoteURL, "://") {
		parsed, err := url.Parse(remoteURL)
		if err != nil {
			return WebHost{}
		}
		// Plain http is kept for self-hosted servers without TLS; ssh and git
		// remotes are served over https.
		if parsed.Scheme == "http" {
			scheme = "http"
		} else if parsed.Scheme != "https" && parsed.Scheme != "ssh" && parsed.Scheme != "git" && parsed.Scheme != "git+ssh" {
			return WebHost{}
		}
		// Hostname drops the port: an ssh port is never the web server's,
		// and an https one is rare enough to give up for simplicity.
		host = parsed.Hostname()
		path = parsed.Path
	} else {
		// scp-like syntax: [user@]host:path. A colon after a slash, or none at
		// all, means a local path (C:\repo has its colon at index 1).
		colon := strings.Index(remoteURL, ":")
		if colon < 2 || strings.Contains(remoteURL[:colon], "/") {
			return WebHost{}
		}
		host = remoteURL[:colon]
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:]
		}
		path = remoteURL[colon+1:]
	}

	host = strings.ToLower(host)
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || !strings.Contains(path, "/") {
		return WebHost{}
	}

	// SSH config aliases such as github.com-work (one per account) stand for
	// the public host.
	for _, public := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		if strings.HasPrefix(host, public+"-") {
			host = public
		}
	}

	var provider string
	switch {
	case strings.Contains(host, "github"):
		provider = "GitHub"
	case strings.Contains(host, "gitlab"):
		provider = "GitLab"
	case host == "bitbucket.org":
		provider = "Bitbucket"
	default:
		return WebHost{}
	}
	return WebHost{Provider: provider, BaseURL: scheme + "://" + host + "/" + path}
}

// resolveWebHost returns the web page of the repository behind the remote the
// branch tracks, or else "origin", or else the first remote by name. The zero
// WebHost is returned when that remote is not on a supported service.
func resolveWebHost(repo *gogit.Repository, branchName string) WebHost {
	cfg, err := repo.Config()
	if err != nil || len(cfg.Remotes) == 0 {
		return WebHost{}
	}

	name := ""
	if branchCfg, ok := cfg.Branches[branchName]; ok && cfg.Remotes[branchCfg.Remote] != nil {
		name = branchCfg.Remote
	} else if cfg.Remotes["origin"] != nil {
		name = "origin"
	} else {
		names := make([]string, 0, len(cfg.Remotes))
		for remoteName := range cfg.Remotes {
			names = append(names, remoteName)
		}
		sort.Strings(names)
		name = names[0]
	}

	urls := cfg.Remotes[name].URLs
	if len(urls) == 0 {
		return WebHost{}
	}
	return ParseWebHost(urls[0])
}
