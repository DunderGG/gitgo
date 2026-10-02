package git_test

import (
	"reflect"
	"strings"
	"testing"

	"gitgo/git"

	"github.com/go-git/go-git/v5/plumbing"
)

var (
	coAuthorAda = git.Trailer{Key: git.CoAuthoredBy, Value: "Ada Lovelace <ada@example.com>"}
	signOffMe   = git.Trailer{Key: git.SignedOffBy, Value: "Test Author <test@example.com>"}
)

// TestTrailerChange_Apply covers adding and removing trailers: where they go,
// duplicates, matching, and messages whose last paragraph is not a trailer
// block.
func TestTrailerChange_Apply(test *testing.T) {
	cases := []struct {
		name    string
		message string
		change  git.TrailerChange
		want    string
	}{
		{
			name:    "adds a paragraph to a subject-only message",
			message: "Fix the parser\n",
			change:  git.TrailerChange{Add: []git.Trailer{coAuthorAda}},
			want:    "Fix the parser\n\nCo-authored-by: Ada Lovelace <ada@example.com>\n",
		},
		{
			name:    "adds a paragraph after a body",
			message: "Fix the parser\n\nIt choked on tabs.\n",
			change:  git.TrailerChange{Add: []git.Trailer{coAuthorAda, signOffMe}},
			want:    "Fix the parser\n\nIt choked on tabs.\n\nCo-authored-by: Ada Lovelace <ada@example.com>\nSigned-off-by: Test Author <test@example.com>\n",
		},
		{
			name:    "appends to an existing block",
			message: "Fix\n\nBody.\n\nReviewed-by: Bob <bob@example.com>\n",
			change:  git.TrailerChange{Add: []git.Trailer{signOffMe}},
			want:    "Fix\n\nBody.\n\nReviewed-by: Bob <bob@example.com>\nSigned-off-by: Test Author <test@example.com>\n",
		},
		{
			name:    "skips a trailer the message has, matching the email",
			message: "Fix\n\nCo-authored-by: Ada L. <ADA@example.com>\n",
			change:  git.TrailerChange{Add: []git.Trailer{coAuthorAda}},
			want:    "Fix\n\nCo-authored-by: Ada L. <ADA@example.com>\n",
		},
		{
			name:    "removes a co-author by email and keeps the others",
			message: "Fix\n\nCo-authored-by: Ada <ada@example.com>\nCo-authored-by: Bob <bob@example.com>\n",
			change:  git.TrailerChange{Remove: []git.Trailer{coAuthorAda}},
			want:    "Fix\n\nCo-authored-by: Bob <bob@example.com>\n",
		},
		{
			name:    "removes every trailer with a key, ignoring case",
			message: "Fix\n\nBody.\n\nsigned-off-by: A <a@example.com>\nSigned-off-by: B <b@example.com>\n",
			change:  git.TrailerChange{Remove: []git.Trailer{{Key: git.SignedOffBy}}},
			want:    "Fix\n\nBody.\n",
		},
		{
			name:    "replaces a trailer",
			message: "Fix\n\nSigned-off-by: Old <old@example.com>\n",
			change:  git.TrailerChange{Remove: []git.Trailer{{Key: git.SignedOffBy}}, Add: []git.Trailer{signOffMe}},
			want:    "Fix\n\nSigned-off-by: Test Author <test@example.com>\n",
		},
		{
			name:    "removes a trailer with continuation lines",
			message: "Fix\n\nNote: a long\n  wrapped note\nCo-authored-by: Ada <ada@example.com>\n",
			change:  git.TrailerChange{Remove: []git.Trailer{{Key: "Note"}}},
			want:    "Fix\n\nCo-authored-by: Ada <ada@example.com>\n",
		},
		{
			name:    "keeps other lines in a block with a sign-off",
			message: "Fix\n\nFixes the crash\nSigned-off-by: Old <old@example.com>\n",
			change:  git.TrailerChange{Add: []git.Trailer{coAuthorAda}},
			want:    "Fix\n\nFixes the crash\nSigned-off-by: Old <old@example.com>\nCo-authored-by: Ada Lovelace <ada@example.com>\n",
		},
		{
			name:    "a last paragraph of prose is not a block",
			message: "Fix\n\nSee: the docs for details\nand more prose here.\n",
			change:  git.TrailerChange{Add: []git.Trailer{signOffMe}},
			want:    "Fix\n\nSee: the docs for details\nand more prose here.\n\nSigned-off-by: Test Author <test@example.com>\n",
		},
		{
			name:    "the subject is never a block",
			message: "Fix: the parser\n",
			change:  git.TrailerChange{Remove: []git.Trailer{{Key: "Fix"}}},
			want:    "Fix: the parser\n",
		},
		{
			name:    "an unchanged message is returned as it was",
			message: "Fix\n\nBody without newline",
			change:  git.TrailerChange{Remove: []git.Trailer{coAuthorAda}},
			want:    "Fix\n\nBody without newline",
		},
		{
			name:    "trailing blank lines are tidied when changed",
			message: "Fix\n\n\n",
			change:  git.TrailerChange{Add: []git.Trailer{signOffMe}},
			want:    "Fix\n\nSigned-off-by: Test Author <test@example.com>\n",
		},
	}
	for _, tc := range cases {
		test.Run(tc.name, func(test *testing.T) {
			if err := tc.change.Validate(); err != nil {
				test.Fatalf("Validate: %v", err)
			}
			if got := tc.change.Apply(tc.message); got != tc.want {
				test.Errorf("Apply =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestParseTrailers verifies that the trailers of the last paragraph are
// returned, with continuation lines joined, and none for prose.
func TestParseTrailers(test *testing.T) {
	got := git.ParseTrailers("Fix\n\nBody.\n\nNote: one\n two\nCo-authored-by: Ada <ada@example.com>\n")
	want := []git.Trailer{{Key: "Note", Value: "one two"}, {Key: git.CoAuthoredBy, Value: "Ada <ada@example.com>"}}
	if !reflect.DeepEqual(got, want) {
		test.Errorf("ParseTrailers = %+v, want %+v", got, want)
	}
	if got := git.ParseTrailers("Fix\n\nJust prose.\n"); got != nil {
		test.Errorf("ParseTrailers(prose) = %+v, want none", got)
	}
}

// TestTrailerChange_Validate verifies that keys and values that would not
// read back as a trailer are refused.
func TestTrailerChange_Validate(test *testing.T) {
	for _, change := range []git.TrailerChange{
		{Add: []git.Trailer{{Key: "Co authored by", Value: "x"}}},
		{Add: []git.Trailer{{Key: "", Value: "x"}}},
		{Add: []git.Trailer{{Key: git.CoAuthoredBy, Value: "Ada\n<ada@example.com>"}}},
		{Add: []git.Trailer{{Key: git.CoAuthoredBy, Value: "  "}}},
		{Remove: []git.Trailer{{Key: "Bad:key"}}},
	} {
		if err := change.Validate(); err == nil {
			test.Errorf("Validate(%+v) = nil, want an error", change)
		}
	}
}

// TestEditCommits_ChangesTrailers verifies that a trailer change is applied
// to each selected commit's message, keeping the rest of it, and is enough on
// its own for an edit.
func TestEditCommits_ChangesTrailers(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	addCommit(test, dir, "first", gitCmd)
	gitCmd("commit", "--allow-empty", "-m", "second\n\nBody.\n\nSigned-off-by: Old <old@example.com>")

	hashes := []plumbing.Hash{revHash(test, dir, "HEAD"), revHash(test, dir, "HEAD~1")}
	err := git.EditCommits(mustOpen(test, dir), hashes, git.BulkEditOptions{
		Trailers: git.TrailerChange{
			Add:    []git.Trailer{coAuthorAda},
			Remove: []git.Trailer{{Key: git.SignedOffBy}},
		},
	})
	if err != nil {
		test.Fatalf("EditCommits: %v", err)
	}

	messages := gitOutputFromDir(test, dir, "git", "log", "--format=%B--", "-3")
	want := strings.Join([]string{
		"second\n\nBody.\n\nCo-authored-by: Ada Lovelace <ada@example.com>\n--",
		"first\n\nCo-authored-by: Ada Lovelace <ada@example.com>\n--",
		"base\n--",
	}, "\n")
	if messages != want {
		test.Errorf("messages =\n%q\nwant\n%q", messages, want)
	}
}

// TestEditCommits_RejectsInvalidTrailer verifies that an invalid trailer is
// refused before anything is written.
func TestEditCommits_RejectsInvalidTrailer(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	before := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD")

	err := git.EditCommits(mustOpen(test, dir), []plumbing.Hash{revHash(test, dir, "HEAD")}, git.BulkEditOptions{
		Trailers: git.TrailerChange{Add: []git.Trailer{{Key: "Not a key", Value: "x"}}},
	})
	if err == nil {
		test.Fatal("EditCommits = nil, want an error")
	}
	if got := gitOutputFromDir(test, dir, "git", "rev-parse", "HEAD"); got != before {
		test.Errorf("HEAD moved from %s to %s", before, got)
	}
}

// TestListAuthors verifies that authors and Co-authored-by co-authors are
// listed once per email, most recently seen first, with the latest name.
func TestListAuthors(test *testing.T) {
	dir := test.TempDir()
	gitCmd := initRepo(test, dir)
	addCommit(test, dir, "base", gitCmd)
	gitCmd("commit", "--allow-empty", "-m", "pair\n\nCo-authored-by: Ada Lovelace <ada@example.com>")
	gitCmd("commit", "--allow-empty", "--author", "Ada L <ADA@example.com>", "-m", "solo")

	authors, err := git.ListAuthors(mustOpen(test, dir))
	if err != nil {
		test.Fatalf("ListAuthors: %v", err)
	}
	want := []git.Identity{
		{Name: "Ada L", Email: "ADA@example.com"},
		{Name: "Test Author", Email: "test@example.com"},
	}
	if !reflect.DeepEqual(authors, want) {
		test.Errorf("ListAuthors = %+v, want %+v", authors, want)
	}
}
