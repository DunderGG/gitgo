package git

import (
	"fmt"
	"regexp"
	"strings"
)

// Trailer keys GitGo offers in its panels. Any key can be added or removed.
const (
	CoAuthoredBy = "Co-authored-by"
	SignedOffBy  = "Signed-off-by"
)

// Trailer is a "Key: value" line at the end of a commit message, such as
// "Co-authored-by: Name <email>".
type Trailer struct {
	Key   string
	Value string
}

// TrailerChange adds and removes trailers in a commit message (see Apply).
type TrailerChange struct {
	// Add is appended to the message's trailer block, skipping each one the
	// message already has.
	Add []Trailer
	// Remove takes out the matching trailers. A Remove with an empty Value
	// takes out every trailer with its key.
	Remove []Trailer
}

// trailerKeyPattern is a trailer key as git interpret-trailers reads one:
// letters, digits and hyphens.
var trailerKeyPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// trailerLinePattern matches the start of a trailer line: a key, optional
// spaces, a colon and the value.
var trailerLinePattern = regexp.MustCompile(`^([A-Za-z0-9-]+)[ \t]*:(.*)$`)

// IsEmpty reports whether change neither adds nor removes anything.
func (change TrailerChange) IsEmpty() bool {
	return len(change.Add) == 0 && len(change.Remove) == 0
}

// Validate rejects a trailer that would not read back as one: a key that is
// not letters, digits and hyphens, a value with a line break, or an empty
// value to add.
func (change TrailerChange) Validate() error {
	for _, trailer := range append(append([]Trailer{}, change.Add...), change.Remove...) {
		if !trailerKeyPattern.MatchString(trailer.Key) {
			return fmt.Errorf("invalid trailer key %q: use only letters, digits and hyphens", trailer.Key)
		}
		if strings.ContainsAny(trailer.Value, "\r\n") {
			return fmt.Errorf("the %s trailer contains a line break", trailer.Key)
		}
	}
	for _, trailer := range change.Add {
		if strings.TrimSpace(trailer.Value) == "" {
			return fmt.Errorf("the %s trailer to add is empty", trailer.Key)
		}
	}
	return nil
}

// Apply returns message with the trailers in change removed and added. The
// trailer block is found the way git interpret-trailers finds it (see
// findTrailerBlock); new trailers go at its end, or in a new last paragraph
// when there is none, and removing every trailer drops the paragraph. A
// message that does not change is returned exactly as it was; a changed one
// ends with a single newline. change must be valid (see Validate).
func (change TrailerChange) Apply(message string) string {
	body, block := splitTrailers(message)
	changed := false

	kept := block[:0:0]
	for _, entry := range block {
		if entry.isTrailer && matchesAny(entry.trailer, change.Remove) {
			changed = true
			continue
		}
		kept = append(kept, entry)
	}
	for _, trailer := range change.Add {
		trailer.Value = strings.TrimSpace(trailer.Value)
		if hasTrailer(kept, trailer) {
			continue
		}
		kept = append(kept, trailerEntry{
			lines:     []string{trailer.Key + ": " + trailer.Value},
			trailer:   trailer,
			isTrailer: true,
		})
		changed = true
	}
	if !changed {
		return message
	}

	var lines []string
	for _, entry := range kept {
		lines = append(lines, entry.lines...)
	}
	switch {
	case len(lines) == 0:
		return body + "\n"
	case body == "":
		return strings.Join(lines, "\n") + "\n"
	default:
		return body + "\n\n" + strings.Join(lines, "\n") + "\n"
	}
}

// ParseTrailers returns the trailers at the end of message, in order.
func ParseTrailers(message string) []Trailer {
	_, block := splitTrailers(message)
	var trailers []Trailer
	for _, entry := range block {
		if entry.isTrailer {
			trailers = append(trailers, entry.trailer)
		}
	}
	return trailers
}

// trailerEntry is one item of a trailer block: a trailer with its
// continuation lines, or a line that is not a trailer (git accepts some in a
// block that has a Signed-off-by), which is kept as it is.
type trailerEntry struct {
	lines     []string
	trailer   Trailer
	isTrailer bool
}

// splitTrailers splits message into the text before its trailer block, with
// trailing blank lines removed, and the block's entries. Without a block, body
// is the whole message (trimmed the same way) and block is empty.
func splitTrailers(message string) (body string, block []trailerEntry) {
	lines := strings.Split(message, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	start := findTrailerBlock(lines)
	if start < 0 {
		return strings.Join(lines, "\n"), nil
	}

	for _, line := range lines[start:] {
		if match := trailerLinePattern.FindStringSubmatch(line); match != nil {
			block = append(block, trailerEntry{
				lines:     []string{line},
				trailer:   Trailer{Key: match[1], Value: strings.TrimSpace(match[2])},
				isTrailer: true,
			})
			continue
		}
		if startsWithSpace(line) && len(block) > 0 && block[len(block)-1].isTrailer {
			last := &block[len(block)-1]
			last.lines = append(last.lines, line)
			last.trailer.Value += " " + strings.TrimSpace(line)
			continue
		}
		block = append(block, trailerEntry{lines: []string{line}})
	}

	bodyLines := lines[:start]
	for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) == "" {
		bodyLines = bodyLines[:len(bodyLines)-1]
	}
	return strings.Join(bodyLines, "\n"), block
}

// findTrailerBlock returns the index of the first line of the trailer block in
// lines (which has no trailing blank lines), or -1 when there is none. Like
// git interpret-trailers, the block is the last paragraph, never the subject
// paragraph, and it counts when all of its lines are trailers or continuation
// lines, or when it has a Signed-off-by or "(cherry picked from commit" line
// and at least a quarter of its lines are trailers.
func findTrailerBlock(lines []string) int {
	start := len(lines)
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	// The first paragraph is the subject (and anything glued to it).
	if start == 0 || start == len(lines) {
		return -1
	}
	hasContent := false
	for _, line := range lines[:start] {
		if strings.TrimSpace(line) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return -1
	}

	trailerLines, otherLines := 0, 0
	recognized := false
	inTrailer := false
	for _, line := range lines[start:] {
		switch {
		case trailerLinePattern.MatchString(line):
			trailerLines++
			inTrailer = true
			if strings.HasPrefix(line, SignedOffBy+": ") {
				recognized = true
			}
		case startsWithSpace(line) && inTrailer:
			trailerLines++
		default:
			otherLines++
			inTrailer = false
			if strings.HasPrefix(line, "(cherry picked from commit ") {
				recognized = true
			}
		}
	}
	if trailerLines > 0 && (otherLines == 0 || (recognized && trailerLines*3 >= otherLines)) {
		return start
	}
	return -1
}

func startsWithSpace(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

// hasTrailer reports whether entries has a trailer matching trailer.
func hasTrailer(entries []trailerEntry, trailer Trailer) bool {
	for _, entry := range entries {
		if entry.isTrailer && trailerMatches(entry.trailer, trailer) {
			return true
		}
	}
	return false
}

// matchesAny reports whether trailer matches one of patterns.
func matchesAny(trailer Trailer, patterns []Trailer) bool {
	for _, pattern := range patterns {
		if trailerMatches(trailer, pattern) {
			return true
		}
	}
	return false
}

// trailerMatches reports whether trailer matches pattern: the keys are equal
// ignoring case, and pattern's value is empty or the same value. Values that
// are identities ("Name <email>") match by email, ignoring case, so a
// co-author is found whatever spelling of their name was used; other values
// are compared with runs of spaces collapsed.
func trailerMatches(trailer, pattern Trailer) bool {
	if !strings.EqualFold(trailer.Key, pattern.Key) {
		return false
	}
	if strings.TrimSpace(pattern.Value) == "" {
		return true
	}
	if identity, ok := ParseIdentity(trailer.Value); ok && identity.Email != "" {
		if want, ok := ParseIdentity(pattern.Value); ok && want.Email != "" {
			return strings.EqualFold(identity.Email, want.Email)
		}
	}
	return strings.Join(strings.Fields(trailer.Value), " ") == strings.Join(strings.Fields(pattern.Value), " ")
}

// identityPattern matches "Name <email>".
var identityPattern = regexp.MustCompile(`^\s*([^<>]*?)\s*<([^<>]*)>\s*$`)

// ParseIdentity reads a "Name <email>" value, as in a Co-authored-by trailer.
// It reports false when value is not in that form.
func ParseIdentity(value string) (Identity, bool) {
	match := identityPattern.FindStringSubmatch(value)
	if match == nil {
		return Identity{}, false
	}
	return Identity{Name: match[1], Email: strings.TrimSpace(match[2])}, true
}
