package app

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGetGitStatus_FindsGit(test *testing.T) {
	// The tests need git anyway (runGit).
	status := New().GetGitStatus()
	if !status.Available || status.Version == "" || status.Problem != "" {
		test.Fatalf("GetGitStatus = %+v, want git available with a version", status)
	}
}

func TestRunHistory_RequiresOpenRepository(test *testing.T) {
	if _, err := New().RunHistory(HistoryOneLine, nil); err == nil {
		test.Fatal("RunHistory without a repository: want an error")
	}
}

func TestHistoryArgs_RejectsUnknownFormatAndHashes(test *testing.T) {
	if _, _, _, err := historyArgs("--output=x", "main", nil); err == nil {
		test.Fatal("unknown format: want an error")
	}
	for _, hash := range []string{"--all", "HEAD", "abc123", strings.Repeat("A", 40)} {
		if _, _, _, err := historyArgs(HistoryOneLine, "main", []string{hash}); err == nil {
			test.Fatalf("hash %q: want an error", hash)
		}
	}
}

func TestRunHistory_WholeList(test *testing.T) {
	_, app := setupRepoWithUnpushedCommit(test)

	result, err := app.RunHistory(HistoryOneLine, nil)
	if err != nil {
		test.Fatalf("RunHistory: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "Test Author: local") || !strings.HasSuffix(lines[1], "Test Author: base") {
		test.Fatalf("output = %q, want local then base", result.Output)
	}
	if !strings.Contains(result.Command, "refs/heads/main") || result.FileName != "history-main.txt" {
		test.Fatalf("result = %+v", result)
	}
}

func TestRunHistory_SelectedInGivenOrder(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")
	base := runGit(test, dir, "rev-parse", "HEAD~1")

	result, err := app.RunHistory(HistoryFull, []string{base, local})
	if err != nil {
		test.Fatalf("RunHistory: %v", err)
	}
	baseAt := strings.Index(result.Output, "commit "+base)
	localAt := strings.Index(result.Output, "commit "+local)
	if baseAt < 0 || localAt < 0 || baseAt > localAt {
		test.Fatalf("output = %q, want base then local", result.Output)
	}
	if !strings.Contains(result.Output, "CommitDate:") {
		test.Fatalf("output = %q, want the fuller format", result.Output)
	}
}

func TestRunHistory_CSVQuotesFields(test *testing.T) {
	dir, app := setupRepoWithUnpushedCommit(test)
	subject := `fix: commas, "quotes" too`
	runGit(test, dir, "commit", "--allow-empty", "-m", subject)
	hash := runGit(test, dir, "rev-parse", "HEAD")

	result, err := app.RunHistory(HistoryCSV, []string{hash})
	if err != nil {
		test.Fatalf("RunHistory: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(result.Output)).ReadAll()
	if err != nil {
		test.Fatalf("parsing CSV %q: %v", result.Output, err)
	}
	if len(records) != 2 || !reflect.DeepEqual(records[0], csvHeader) {
		test.Fatalf("records = %q, want a header and one commit", records)
	}
	commit := records[1]
	if commit[0] != hash || commit[1] != "Test Author" || commit[2] != "test@example.com" || commit[7] != subject {
		test.Fatalf("record = %q", commit)
	}
	if result.FileName != "history-main-selected.csv" {
		test.Fatalf("FileName = %q", result.FileName)
	}
}

func TestExportPatches_OldestFirst(test *testing.T) {
	dir, _ := setupRepoWithUnpushedCommit(test)
	local := runGit(test, dir, "rev-parse", "HEAD")
	base := runGit(test, dir, "rev-parse", "HEAD~1")
	out := filepath.Join(test.TempDir(), "patch folder")

	result, err := exportPatches(dir, []string{local, base}, out)
	if err != nil {
		test.Fatalf("exportPatches: %v", err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		test.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := []string{"0001-base.patch", "0002-local.patch"}
	if !reflect.DeepEqual(names, want) {
		test.Fatalf("files = %v, want %v", names, want)
	}
	if !strings.Contains(result.Output, "0001-base.patch\n0002-local.patch\n") {
		test.Fatalf("output = %q", result.Output)
	}
	// Exporting must not change the repository.
	if head := runGit(test, dir, "rev-parse", "HEAD"); head != local {
		test.Fatalf("HEAD moved to %s", head)
	}
}

func TestExportPatches_SkipsMergeCommits(test *testing.T) {
	dir, _ := setupRepoWithUnpushedCommit(test)
	runGit(test, dir, "checkout", "-b", "side", "HEAD~1")
	commitFile(test, dir, "side")
	runGit(test, dir, "checkout", "main")
	runGit(test, dir, "merge", "--no-ff", "-m", "merge side", "side")
	merge := runGit(test, dir, "rev-parse", "HEAD")

	result, err := exportPatches(dir, []string{merge}, test.TempDir())
	if err != nil {
		test.Fatalf("exportPatches: %v", err)
	}
	if !strings.Contains(result.Output, "Wrote 0 patch file(s)") || !strings.Contains(result.Output, merge[:7]) {
		test.Fatalf("output = %q, want the merge reported as skipped", result.Output)
	}
}

func TestCappedBuffer_KeepsFirstBytes(test *testing.T) {
	capped := &cappedBuffer{limit: 5}
	for _, chunk := range []string{"abc", "def", "ghi"} {
		if n, err := capped.Write([]byte(chunk)); n != len(chunk) || err != nil {
			test.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if got := capped.buffer.String(); got != "abcde" || !capped.truncated {
		test.Fatalf("buffer = %q, truncated = %v", got, capped.truncated)
	}
}
