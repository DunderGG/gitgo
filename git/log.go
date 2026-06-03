package git

import (
	"fmt"
	"io"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const defaultLogDepth = 100

// CommitEntry is the git-layer representation of a single commit.
// The app layer converts this to app.CommitSummary.
type CommitEntry struct {
	Hash       plumbing.Hash
	ShortHash  string
	Message    string
	AuthorName string
	Date       time.Time
	IsUnpushed bool
}

// Log walks the commit history from HEAD and returns up to limit entries.
// Each entry is annotated with IsUnpushed based on the precomputed set in
// state.UnpushedHashes. If limit is <= 0 the defaultLogDepth is used.
func Log(state *RepoState, limit int) ([]CommitEntry, error) {
	if limit <= 0 {
		limit = defaultLogDepth
	}

	head, err := state.Repo.Head()
	if err != nil {
		return nil, fmt.Errorf("reading HEAD: %w", err)
	}

	logIter, err := state.Repo.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("opening log: %w", err)
	}
	defer logIter.Close()

	// Pre-allocate with the expected capacity to avoid repeated slice growth.
	entries := make([]CommitEntry, 0, limit)
	for len(entries) < limit {
		commit, iterErr := logIter.Next()
		if iterErr == io.EOF {
			// No more commits in the repo — this is the normal exit path.
			break
		}
		if iterErr != nil {
			return nil, fmt.Errorf("iterating log: %w", iterErr)
		}

		hash := commit.Hash
		entries = append(entries, CommitEntry{
			Hash:      hash,
			ShortHash: hash.String()[:7], // first 7 hex chars, same as git's default short hash
			Message:   firstLine(commit.Message), // only the subject line; body is available via GetCommitDetail
			AuthorName: commit.Author.Name,
			Date:       commit.Author.When,
			// Look up in the precomputed set built by Open() — O(1) per entry.
			IsUnpushed: state.UnpushedHashes[hash],
		})
	}

	return entries, nil
}

// firstLine returns the first line of s, trimming any trailing whitespace.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
