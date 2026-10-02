package app

import (
	"fmt"

	gitpkg "gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

// ChangeTrailers returns message with the trailers in change removed and
// added, for the buttons under the message field in the edit panel. Nothing
// is written; the panel shows the new message for the user to apply.
func (app *App) ChangeTrailers(message string, change TrailerChange) (string, error) {
	trailers := gitTrailerChange(change)
	if err := trailers.Validate(); err != nil {
		return "", err
	}
	return trailers.Apply(message), nil
}

// PreviewTrailers returns each commit's trailers before and after change, for
// the bulk panel's preview. EditCommits applies the same change the same way,
// so what is applied is what was shown.
func (app *App) PreviewTrailers(hashes []string, change TrailerChange) ([]TrailerPreview, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}
	trailers := gitTrailerChange(change)
	if err := trailers.Validate(); err != nil {
		return nil, err
	}

	previews := make([]TrailerPreview, 0, len(hashes))
	for _, hash := range hashes {
		commit, err := state.Repo.CommitObject(plumbing.NewHash(hash))
		if err != nil {
			return nil, fmt.Errorf("commit %s not found: %w", hash, err)
		}
		after := trailers.Apply(commit.Message)
		previews = append(previews, TrailerPreview{
			Hash:    hash,
			Before:  appTrailers(gitpkg.ParseTrailers(commit.Message)),
			After:   appTrailers(gitpkg.ParseTrailers(after)),
			Changed: after != commit.Message,
		})
	}
	return previews, nil
}

// ListAuthors returns the people in the branch's history, most recently seen
// first, for the co-author picker (see git.ListAuthors).
func (app *App) ListAuthors() ([]Identity, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}
	authors, err := gitpkg.ListAuthors(state)
	if err != nil {
		return nil, err
	}
	result := make([]Identity, len(authors))
	for i, author := range authors {
		result[i] = Identity{Name: author.Name, Email: author.Email}
	}
	return result, nil
}

// gitTrailerChange maps a TrailerChange to a git.TrailerChange.
func gitTrailerChange(change TrailerChange) gitpkg.TrailerChange {
	convert := func(trailers []Trailer) []gitpkg.Trailer {
		var result []gitpkg.Trailer
		for _, trailer := range trailers {
			result = append(result, gitpkg.Trailer{Key: trailer.Key, Value: trailer.Value})
		}
		return result
	}
	return gitpkg.TrailerChange{Add: convert(change.Add), Remove: convert(change.Remove)}
}

// appTrailers maps git.Trailers to the Trailer DTOs, never nil so the
// frontend always gets a list.
func appTrailers(trailers []gitpkg.Trailer) []Trailer {
	result := make([]Trailer, len(trailers))
	for i, trailer := range trailers {
		result[i] = Trailer{Key: trailer.Key, Value: trailer.Value}
	}
	return result
}
