package app

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	gitpkg "gitgo/git"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The Run menu runs the native git program with arguments built here, never
// with text from the user, and only commands that read the repository, so
// nothing in the menu can change it. git is otherwise not a runtime
// dependency: edits use go-git.

// History formats for RunHistory.
const (
	HistoryOneLine = "oneline"
	HistoryFull    = "full"
	HistoryCSV     = "csv"
)

// maxRunOutput is the most output kept from one command, so a huge history
// cannot exhaust memory or freeze the dialog showing it.
const maxRunOutput = 10 << 20

// gitMissingProblem explains the disabled Run menu when git is not found.
const gitMissingProblem = "Git is not installed or not on PATH. The Run menu runs the git program, which GitGo needs for nothing else."

// commitHashPattern matches a full SHA-1 or SHA-256 commit hash.
var commitHashPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// gitConfigArgs come before every subcommand: output in UTF-8, and no gpg
// runs to verify signatures when log.showSignature is set.
var gitConfigArgs = []string{"-c", "i18n.logOutputEncoding=UTF-8", "-c", "log.showSignature=false"}

// gitCommand builds the command that runs git with args in dir, without a
// console window and without prompts.
func gitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append(append([]string{}, gitConfigArgs...), args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		// Do not take locks or refresh the index while reading.
		"GIT_OPTIONAL_LOCKS=0",
	)
	hideConsole(cmd)
	return cmd
}

// cappedBuffer keeps the first limit bytes written to it and drops the rest,
// so the command still runs to its end.
type cappedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (capped *cappedBuffer) Write(data []byte) (int, error) {
	room := capped.limit - capped.buffer.Len()
	if len(data) > room {
		capped.truncated = true
		if room > 0 {
			capped.buffer.Write(data[:room])
		}
		return len(data), nil
	}
	capped.buffer.Write(data)
	return len(data), nil
}

// runGitCommand runs git with args in dir, passing stdin, and returns its
// output, keeping at most maxRunOutput bytes. A failure includes what git
// printed on stderr.
func runGitCommand(dir string, stdin string, args ...string) ([]byte, bool, error) {
	cmd := gitCommand(dir, args...)
	output := &cappedBuffer{limit: maxRunOutput}
	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, false, errors.New(gitMissingProblem)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, false, fmt.Errorf("git %s failed: %s", args[0], message)
	}
	return output.buffer.Bytes(), output.truncated, nil
}

// GetGitStatus reports whether the git program the Run menu needs is
// installed, and its version.
func (app *App) GetGitStatus() GitStatus {
	if _, err := exec.LookPath("git"); err != nil {
		return GitStatus{Problem: gitMissingProblem}
	}
	output, err := gitCommand("", "--version").Output()
	if err != nil {
		return GitStatus{Problem: fmt.Sprintf("git was found but did not run: %v", err)}
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output)), "git version ")
	return GitStatus{Available: true, Version: version}
}

// openState returns the open repository, or an error when there is none.
func (app *App) openState() (*gitpkg.RepoState, error) {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return nil, fmt.Errorf("no repository is open; call OpenRepository first")
	}
	return state, nil
}

// checkHashes rejects anything but full commit hashes, so the hashes cannot
// be read as options or revision expressions.
func checkHashes(hashes []string) error {
	for _, hash := range hashes {
		if !commitHashPattern.MatchString(hash) {
			return fmt.Errorf("%q is not a full commit hash", hash)
		}
	}
	return nil
}

// historyFormatArgs are the git log options for each history format. CSV
// separates fields with US (0x1f) and commits with RS (0x1e), which
// historyCSV turns into quoted CSV.
var historyFormatArgs = map[string][]string{
	HistoryOneLine: {"--format=%h %ad %an: %s", "--date=short"},
	HistoryFull:    {"--format=fuller", "--date=iso"},
	HistoryCSV:     {"--format=%H%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%s%x1e"},
}

// csvHeader names the columns of the CSV history.
var csvHeader = []string{"hash", "author", "author email", "author date", "committer", "committer email", "committer date", "subject"}

// historyArgs builds the git log arguments for format over hashes, in the
// order given, or over the commits the commit list shows on branch when
// hashes is empty. Hashes are passed on stdin, which has no length limit;
// display is the equivalent command with them on the command line.
func historyArgs(format string, branch string, hashes []string) (args []string, stdin string, display []string, err error) {
	formatArgs, ok := historyFormatArgs[format]
	if !ok {
		return nil, "", nil, fmt.Errorf("unknown history format %q", format)
	}
	if err := checkHashes(hashes); err != nil {
		return nil, "", nil, err
	}

	args = append([]string{"log", "--no-color"}, formatArgs...)
	if len(hashes) == 0 {
		// The full ref name cannot be mistaken for an option or a path.
		revisions := []string{fmt.Sprintf("--max-count=%d", gitpkg.DefaultLogDepth), "refs/heads/" + branch, "--"}
		args = append(args, revisions...)
		return args, "", args, nil
	}

	display = append(append([]string{}, args...), "--no-walk=unsorted")
	for _, hash := range hashes {
		display = append(display, hash[:7])
	}
	args = append(args, "--no-walk=unsorted", "--stdin")
	return args, strings.Join(hashes, "\n") + "\n", display, nil
}

// historyCSV turns the US / RS separated output of the CSV format into CSV
// with a header row. A record cut off by truncation is dropped.
func historyCSV(output []byte) (string, error) {
	records := strings.Split(string(output), "\x1e")
	// The text after the last separator is the final newline, or a record
	// cut off by truncation.
	records = records[:len(records)-1]

	var text strings.Builder
	writer := csv.NewWriter(&text)
	if err := writer.Write(csvHeader); err != nil {
		return "", err
	}
	for _, record := range records {
		fields := strings.Split(strings.TrimLeft(record, "\n"), "\x1f")
		if len(fields) != len(csvHeader) {
			return "", fmt.Errorf("unexpected git log output: %q", record)
		}
		if err := writer.Write(fields); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return text.String(), writer.Error()
}

// trimToLastLine cuts truncated output back to its last complete line.
func trimToLastLine(output []byte) []byte {
	if end := bytes.LastIndexByte(output, '\n'); end >= 0 {
		return output[:end+1]
	}
	return output
}

// displayCommand joins args into a command line to show, quoting arguments
// with spaces.
func displayCommand(args []string) string {
	parts := []string{"git"}
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\"") {
			arg = `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}

// unsafeFileNameChars are replaced in suggested file names.
var unsafeFileNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// RunHistory runs git log over the commits with the given hashes, in the
// order given (the commit list's), or over the commits the commit list shows
// when hashes is empty, and returns its output in format: HistoryOneLine,
// HistoryFull or HistoryCSV.
// OpenRepository must be called before this method.
func (app *App) RunHistory(format string, hashes []string) (RunResult, error) {
	state, err := app.openState()
	if err != nil {
		return RunResult{}, err
	}
	args, stdin, display, err := historyArgs(format, state.Branch, hashes)
	if err != nil {
		return RunResult{}, err
	}

	output, truncated, err := runGitCommand(state.Path, stdin, args...)
	if err != nil {
		return RunResult{}, err
	}

	extension := ".txt"
	text := ""
	if format == HistoryCSV {
		extension = ".csv"
		if text, err = historyCSV(output); err != nil {
			return RunResult{}, err
		}
	} else {
		if truncated {
			output = trimToLastLine(output)
		}
		text = string(output)
	}

	name := "history-" + unsafeFileNameChars.ReplaceAllString(state.Branch, "-")
	if len(hashes) > 0 {
		name += "-selected"
	}
	return RunResult{
		Command:   displayCommand(display),
		Output:    text,
		Truncated: truncated,
		FileName:  name + extension,
	}, nil
}

// exportPatches writes one patch per commit into dir with git format-patch,
// numbered from the oldest commit. hashes are in commit list order, newest
// first. Merge commits get no patch, as with git format-patch itself.
func exportPatches(repoPath string, hashes []string, dir string) (RunResult, error) {
	if len(hashes) == 0 {
		return RunResult{}, errors.New("select the commits to export")
	}
	if err := checkHashes(hashes); err != nil {
		return RunResult{}, err
	}

	var files, skipped, commands []string
	for index := len(hashes) - 1; index >= 0; index-- {
		hash := hashes[index]
		number := fmt.Sprintf("--start-number=%d", len(files)+1)
		// --no-walk keeps -1 from walking past a merge commit to its parent.
		args := []string{"format-patch", "--output-directory", dir, number, "-1", "--no-walk", hash}
		commands = append(commands, displayCommand([]string{"format-patch", "-o", dir, number, "-1", "--no-walk", hash[:7]}))
		output, _, err := runGitCommand(repoPath, "", args...)
		if err != nil {
			return RunResult{}, err
		}
		// One path per line; paths may contain spaces.
		var written []string
		if text := strings.TrimSpace(string(output)); text != "" {
			written = strings.Split(text, "\n")
		}
		if len(written) == 0 {
			skipped = append(skipped, hash[:7])
		}
		for _, path := range written {
			files = append(files, filepath.Base(strings.TrimSpace(path)))
		}
	}

	var text strings.Builder
	fmt.Fprintf(&text, "Wrote %d patch file(s) to %s:\n\n", len(files), dir)
	for _, file := range files {
		fmt.Fprintf(&text, "%s\n", file)
	}
	if len(skipped) > 0 {
		fmt.Fprintf(&text, "\nNo patch for %s: git format-patch skips merge commits.\n", strings.Join(skipped, ", "))
	}
	return RunResult{
		Command:  strings.Join(commands, "\n"),
		Output:   text.String(),
		FileName: "format-patch.txt",
	}, nil
}

// ExportPatches asks for a folder in a native dialog and writes one patch per
// commit with the given hashes into it with git format-patch, oldest first.
// hashes are in commit list order, newest first. It returns an empty result
// when the dialog is cancelled.
// OpenRepository must be called before this method.
func (app *App) ExportPatches(hashes []string) (RunResult, error) {
	state, err := app.openState()
	if err != nil {
		return RunResult{}, err
	}
	if len(hashes) == 0 {
		return RunResult{}, errors.New("select the commits to export")
	}
	dir, err := runtime.OpenDirectoryDialog(app.ctx, runtime.OpenDialogOptions{
		Title:                "Choose a folder for the patches",
		CanCreateDirectories: true,
	})
	if err != nil || dir == "" {
		return RunResult{}, err
	}
	return exportPatches(state.Path, hashes, dir)
}

// SaveRunOutput asks for a file in a native save dialog, suggesting
// fileName, and writes content to it. It returns the path written, or an
// empty string when the dialog is cancelled.
func (app *App) SaveRunOutput(fileName string, content string) (string, error) {
	path, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "Save output",
		DefaultFilename: fileName,
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("could not save the output: %w", err)
	}
	return path, nil
}
