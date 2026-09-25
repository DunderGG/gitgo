package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Reflog messages written by GitGo, shown by `git reflog`.
const (
	reflogEditPrefix = "gitgo: edit commit "
	reflogUndo       = "gitgo: undo edit"
)

// moveBranch moves branch from oldHash to newHash and records the move in the
// reflog, so `git reflog` can recover the previous tip just as it can after
// `git commit --amend` or `git rebase`. go-git does not write reflogs itself.
//
// The entry is appended to the branch's reflog, and also to HEAD's when HEAD
// points at the branch. As in git, the reflog is written before the ref: if it
// cannot be written, the branch is not moved.
//
// The ref update is a compare-and-swap: if the branch no longer points at
// oldHash (or no longer exists), it is left alone and ErrBranchChanged is
// returned.
func moveBranch(state *RepoState, branch plumbing.ReferenceName, oldHash, newHash plumbing.Hash, message string) error {
	current, err := state.Repo.Storer.Reference(branch)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return ErrBranchChanged
	}
	if err != nil {
		return fmt.Errorf("reading branch %s: %w", branch.Short(), err)
	}
	if current.Hash() != oldHash {
		return ErrBranchChanged
	}

	if err := appendReflogs(state, branch, oldHash, newHash, message); err != nil {
		return fmt.Errorf("writing reflog: %w", err)
	}

	// CheckAndSetReference only writes the new ref when the stored ref still
	// matches the old one, guarding against a concurrent change on disk.
	newRef := plumbing.NewHashReference(branch, newHash)
	oldRef := plumbing.NewHashReference(branch, oldHash)
	if err := state.Repo.Storer.CheckAndSetReference(newRef, oldRef); err != nil {
		return fmt.Errorf("updating branch ref: %w", err)
	}
	return nil
}

// appendReflogs writes one reflog line for branch, and one for HEAD when HEAD
// is a symbolic ref to branch. Repositories not stored on disk have no reflog
// and are skipped.
func appendReflogs(state *RepoState, branch plumbing.ReferenceName, oldHash, newHash plumbing.Hash, message string) error {
	storage, ok := state.Repo.Storer.(*filesystem.Storage)
	if !ok {
		return nil
	}
	gitDir := storage.Filesystem().Root()

	cfg, err := state.Repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	line := reflogLine(cfg, oldHash, newHash, message, time.Now())

	// core.logAllRefUpdates defaults to true in repositories with a working
	// tree; when it is false git only appends to reflogs that already exist.
	create := cfg.Raw.Section("core").Option("logallrefupdates") != "false"

	names := []plumbing.ReferenceName{branch}
	head, err := state.Repo.Storer.Reference(plumbing.HEAD)
	if err == nil && head.Type() == plumbing.SymbolicReference && head.Target() == branch {
		names = append(names, plumbing.HEAD)
	}

	for _, name := range names {
		path := filepath.Join(gitDir, "logs", filepath.FromSlash(name.String()))
		if err := appendLine(path, line, create); err != nil {
			return err
		}
	}
	return nil
}

// reflogLine formats a reflog entry the way git does:
//
//	<old> <new> <name> <<email>> <unix time> <+hhmm>\t<message>
//
// The identity is the one git would use for a commit: GIT_COMMITTER_NAME /
// GIT_COMMITTER_EMAIL, then user.name / user.email from the config.
func reflogLine(cfg *config.Config, oldHash, newHash plumbing.Hash, message string, now time.Time) string {
	name := os.Getenv("GIT_COMMITTER_NAME")
	if name == "" {
		name = cfg.User.Name
	}
	if name == "" {
		name = "GitGo"
	}
	email := os.Getenv("GIT_COMMITTER_EMAIL")
	if email == "" {
		email = cfg.User.Email
	}

	// A reflog entry is a single line; keep the message on it.
	message = strings.Join(strings.Fields(message), " ")

	return fmt.Sprintf("%s %s %s <%s> %d %s\t%s\n",
		oldHash, newHash, name, email, now.Unix(), now.Format("-0700"), message)
}

// appendLine appends line to the file at path. When create is false the line
// is only written if the file already exists.
func appendLine(path, line string, create bool) error {
	flags := os.O_WRONLY | os.O_APPEND
	if create {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		flags |= os.O_CREATE
	} else if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(line); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
