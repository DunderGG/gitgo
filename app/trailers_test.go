package app

import (
	"reflect"
	"testing"
)

// TestPreviewTrailers_PreviewIsApplied verifies that PreviewTrailers shows
// each commit's trailers before and after a change, and that EditCommits with
// the same change gives the commits exactly the previewed trailers.
func TestPreviewTrailers_PreviewIsApplied(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	runGit(test, dir, "commit", "--allow-empty", "-m", "second\n\nSigned-off-by: Old <old@example.com>")
	if _, err := app.ReloadRepository(); err != nil {
		test.Fatalf("ReloadRepository: %v", err)
	}
	hashes := []string{runGit(test, dir, "rev-parse", "HEAD"), runGit(test, dir, "rev-parse", "HEAD~1")}
	adaTrailer := Trailer{Key: "Co-authored-by", Value: "Ada <ada@example.com>"}
	change := TrailerChange{
		Add:    []Trailer{adaTrailer},
		Remove: []Trailer{{Key: "Signed-off-by"}},
	}

	previews, err := app.PreviewTrailers(hashes, change)
	if err != nil {
		test.Fatalf("PreviewTrailers: %v", err)
	}
	want := []TrailerPreview{
		{Hash: hashes[0], Before: []Trailer{{Key: "Signed-off-by", Value: "Old <old@example.com>"}}, After: []Trailer{adaTrailer}, Changed: true},
		{Hash: hashes[1], Before: []Trailer{}, After: []Trailer{adaTrailer}, Changed: true},
	}
	if !reflect.DeepEqual(previews, want) {
		test.Fatalf("PreviewTrailers = %+v, want %+v", previews, want)
	}

	if result, err := app.EditCommits(BulkEditRequest{Hashes: hashes, Trailers: change}); err != nil || !result.Success {
		test.Fatalf("EditCommits = %+v, %v", result, err)
	}
	if got := runGit(test, dir, "log", "-2", "--format=%(trailers:only,unfold)%s"); got != "Co-authored-by: Ada <ada@example.com>\nsecond\nCo-authored-by: Ada <ada@example.com>\nlocal" {
		test.Errorf("trailers and subjects after the edit = %q", got)
	}
}

// TestChangeTrailers_ReturnsNewMessage verifies the single-commit helper and
// that it refuses an invalid trailer.
func TestChangeTrailers_ReturnsNewMessage(test *testing.T) {
	app := New()
	got, err := app.ChangeTrailers("Fix\n", TrailerChange{Add: []Trailer{{Key: "Signed-off-by", Value: "Me <me@example.com>"}}})
	if err != nil || got != "Fix\n\nSigned-off-by: Me <me@example.com>\n" {
		test.Errorf("ChangeTrailers = %q, %v", got, err)
	}
	if _, err := app.ChangeTrailers("Fix\n", TrailerChange{Add: []Trailer{{Key: "Bad key", Value: "x"}}}); err == nil {
		test.Error("ChangeTrailers with an invalid key = nil error, want an error")
	}
}

// TestListAuthors_ListsBranchAuthors verifies the bound method returns the
// branch's authors.
func TestListAuthors_ListsBranchAuthors(test *testing.T) {
	_, app := setupRepoWithUnpushedCommit(test)
	authors, err := app.ListAuthors()
	if err != nil {
		test.Fatalf("ListAuthors: %v", err)
	}
	if want := []Identity{{Name: "Test Author", Email: "test@example.com"}}; !reflect.DeepEqual(authors, want) {
		test.Errorf("ListAuthors = %+v, want %+v", authors, want)
	}
}
