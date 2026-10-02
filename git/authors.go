package git

import (
	"fmt"
	"io"
	"strings"

	gogit "github.com/go-git/go-git/v5"
)

// authorsDepth is how many commits ListAuthors reads.
const authorsDepth = 2000

// ListAuthors returns the people in the history of state.Branch, for picking
// co-authors: the authors of its newest commits (up to authorsDepth) and the
// co-authors in their Co-authored-by trailers, most recently seen first. Each
// email appears once, with the name it was last used with; identities without
// an email are listed by name.
func ListAuthors(state *RepoState) ([]Identity, error) {
	head, err := branchTip(state)
	if err != nil {
		return nil, err
	}
	logIter, err := state.Repo.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("opening log: %w", err)
	}
	defer logIter.Close()

	var authors []Identity
	seen := make(map[string]bool)
	add := func(identity Identity) {
		identity.Name = strings.TrimSpace(identity.Name)
		identity.Email = strings.TrimSpace(identity.Email)
		if identity.Name == "" {
			return
		}
		key := "email:" + strings.ToLower(identity.Email)
		if identity.Email == "" {
			key = "name:" + strings.ToLower(identity.Name)
		}
		if !seen[key] {
			seen[key] = true
			authors = append(authors, identity)
		}
	}

	for count := 0; count < authorsDepth; count++ {
		commit, err := logIter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterating log: %w", err)
		}
		add(Identity{Name: commit.Author.Name, Email: commit.Author.Email})
		for _, trailer := range ParseTrailers(commit.Message) {
			if !strings.EqualFold(trailer.Key, CoAuthoredBy) {
				continue
			}
			if identity, ok := ParseIdentity(trailer.Value); ok {
				add(identity)
			}
		}
	}
	return authors, nil
}
