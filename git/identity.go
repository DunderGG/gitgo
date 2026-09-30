package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/config"
)

// ConfiguredIdentity returns the author identity git would use for a new
// commit in the repository: GIT_AUTHOR_NAME / GIT_AUTHOR_EMAIL, then user.name
// / user.email from the repository and global config. Either field is empty
// when it is not set anywhere.
func ConfiguredIdentity(state *RepoState) (Identity, error) {
	cfg, err := state.Repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return Identity{}, fmt.Errorf("reading config: %w", err)
	}
	identity := Identity{Name: os.Getenv("GIT_AUTHOR_NAME"), Email: os.Getenv("GIT_AUTHOR_EMAIL")}
	if identity.Name == "" {
		identity.Name = cfg.User.Name
	}
	if identity.Email == "" {
		identity.Email = cfg.User.Email
	}
	return identity, nil
}
