package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// SignedCommit is a signed commit that a rewrite would rebuild, and so leave
// unsigned: rebuildCommit drops signatures because they cover the original
// commit bytes and would no longer verify.
type SignedCommit struct {
	Hash plumbing.Hash
	// Subject is the first line of the commit message.
	Subject string
	// Edited is true for the commits being edited, false for the commits
	// above them that are only rebuilt with new parents.
	Edited bool
}

// FindSignedCommits returns the signed commits in the chain an edit of the
// targets would rebuild (the first-parent chain from the branch tip down to
// the oldest target), newest first.
func FindSignedCommits(state *RepoState, targets ...plumbing.Hash) ([]SignedCommit, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no commits to check")
	}
	tip, err := branchTip(state)
	if err != nil {
		return nil, err
	}
	chain, err := collectChain(state, tip.Hash(), targets...)
	if err != nil {
		return nil, err
	}

	edited := make(map[plumbing.Hash]bool, len(targets))
	for _, target := range targets {
		edited[target] = true
	}

	var signed []SignedCommit
	for _, commit := range chain {
		isSigned, err := hasSignature(state, commit)
		if err != nil {
			return nil, err
		}
		if isSigned {
			subject, _, _ := strings.Cut(commit.Message, "\n")
			signed = append(signed, SignedCommit{Hash: commit.Hash, Subject: subject, Edited: edited[commit.Hash]})
		}
	}
	return signed, nil
}

// hasSignature reports whether commit carries a gpgsig or gpgsig-sha256
// header (GPG, SSH and X.509 signatures all use these). go-git only parses
// gpgsig into PGPSignature and drops gpgsig-sha256, so the raw headers are
// checked when PGPSignature is empty.
func hasSignature(state *RepoState, commit *object.Commit) (bool, error) {
	if commit.PGPSignature != "" {
		return true, nil
	}

	encoded, err := state.Repo.Storer.EncodedObject(plumbing.CommitObject, commit.Hash)
	if err != nil {
		return false, fmt.Errorf("loading commit %s: %w", commit.Hash, err)
	}
	reader, err := encoded.Reader()
	if err != nil {
		return false, fmt.Errorf("reading commit %s: %w", commit.Hash, err)
	}
	defer reader.Close()

	// Headers end at the first empty line; the message follows.
	lines := bufio.NewReader(reader)
	for {
		line, err := lines.ReadBytes('\n')
		if bytes.HasPrefix(line, []byte("gpgsig ")) || bytes.HasPrefix(line, []byte("gpgsig-sha256 ")) {
			return true, nil
		}
		if len(bytes.TrimRight(line, "\n")) == 0 || err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("reading commit %s: %w", commit.Hash, err)
		}
	}
}
