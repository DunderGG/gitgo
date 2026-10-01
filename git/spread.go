package git

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// ErrInvalidSpreadRange is returned by SpreadDates when the last date is not
// after the first, or when even spacing would need gaps under one second.
var ErrInvalidSpreadRange = errors.New("invalid date range")

// Spacing says how SpreadDates places the commits between the first and
// last date.
type Spacing int

const (
	// KeepSpacing scales the gaps between the commits' current author dates
	// to the new range, so commits made close together stay close together.
	KeepSpacing Spacing = iota
	// EvenSpacing puts the same gap between every commit.
	EvenSpacing
	// RandomSpacing places the commits at random, never closer together
	// than SpreadOptions.MinGap.
	RandomSpacing
)

// SpreadOptions describes how SpreadDates fits commits into a date range.
type SpreadOptions struct {
	// First and Last are the new author dates of the oldest and newest
	// commit. Only the moment counts; each commit keeps its own offset.
	First time.Time
	Last  time.Time
	// Spacing places the commits in between.
	Spacing Spacing
	// MinGap is the least time between two neighbouring commits with
	// RandomSpacing, in whole seconds.
	MinGap time.Duration
	// Seed makes RandomSpacing repeatable, so the preview shown to the user
	// and the dates applied are the same.
	Seed uint64
}

// SpreadResult holds the new author dates computed by SpreadDates.
type SpreadResult struct {
	// Dates is each commit's new author date, in its original offset.
	Dates map[plumbing.Hash]time.Time
	// FellBack is set when KeepSpacing was asked for but the commits were
	// spaced evenly, because their current dates are all equal or not in
	// history order.
	FellBack bool
}

// SpreadDates computes new author dates that fit the commits in hashes
// between opts.First and opts.Last. The commits are ordered by their position
// in the branch's first-parent history, not by their current dates: the
// oldest gets opts.First and the newest opts.Last, exactly. Dates are whole
// seconds, as git stores them, and never out of history order.
//
// Nothing is written; pass the result to EditCommits as BulkEditOptions.Dates.
//
// Returns ErrInvalidSpreadRange when opts.Last is not after opts.First (with
// more than one commit), when even spacing needs gaps under one second, or
// when the range is too short for random spacing's minimum gaps.
func SpreadDates(state *RepoState, hashes []plumbing.Hash, opts SpreadOptions) (SpreadResult, error) {
	if len(hashes) == 0 {
		return SpreadResult{}, fmt.Errorf("no commits to spread")
	}
	head, err := branchTip(state)
	if err != nil {
		return SpreadResult{}, err
	}
	chain, err := collectChain(state, head.Hash(), hashes...)
	if err != nil {
		return SpreadResult{}, err
	}

	// chain is newest first; keep the selected commits, oldest first.
	selected := make(map[plumbing.Hash]bool, len(hashes))
	for _, hash := range hashes {
		selected[hash] = true
	}
	var ordered []plumbing.Hash
	var original []time.Time
	for i := len(chain) - 1; i >= 0; i-- {
		if selected[chain[i].Hash] {
			ordered = append(ordered, chain[i].Hash)
			original = append(original, chain[i].Author.When)
		}
	}

	spread, fellBack, err := spreadTimes(original, opts)
	if err != nil {
		return SpreadResult{}, err
	}
	dates := make(map[plumbing.Hash]time.Time, len(ordered))
	for i, hash := range ordered {
		dates[hash] = spread[i]
	}
	return SpreadResult{Dates: dates, FellBack: fellBack}, nil
}

// spreadTimes is the calculation behind SpreadDates, for original author
// dates listed oldest commit first. Each result keeps the location (offset)
// of its original date.
func spreadTimes(original []time.Time, opts SpreadOptions) ([]time.Time, bool, error) {
	count := len(original)
	first, last := opts.First.Unix(), opts.Last.Unix()
	if count == 1 {
		return []time.Time{time.Unix(first, 0).In(original[0].Location())}, false, nil
	}
	if last <= first {
		return nil, false, fmt.Errorf("%w: the last date must be after the first", ErrInvalidSpreadRange)
	}

	span := last - first
	seconds := make([]int64, count)
	fellBack := false
	switch {
	case opts.Spacing == KeepSpacing && inHistoryOrder(original):
		// Scale each commit's distance from the oldest one to the new range.
		oldest, originalSpan := original[0].Unix(), float64(original[count-1].Unix()-original[0].Unix())
		for i, date := range original {
			seconds[i] = first + int64(math.Round(float64(date.Unix()-oldest)/originalSpan*float64(span)))
		}
	case opts.Spacing == RandomSpacing:
		if err := randomSeconds(seconds, first, span, opts); err != nil {
			return nil, false, err
		}
	default:
		fellBack = opts.Spacing == KeepSpacing
		if span < int64(count-1) {
			return nil, false, fmt.Errorf("%w: %d commits need at least %s between the first and last date",
				ErrInvalidSpreadRange, count, formatSpan(int64(count-1)))
		}
		gap := float64(span) / float64(count-1)
		for i := range seconds {
			seconds[i] = first + int64(math.Round(float64(i)*gap))
		}
	}
	// The ends are exact even where rounding could have moved them.
	seconds[0], seconds[count-1] = first, last

	result := make([]time.Time, count)
	for i, second := range seconds {
		result[i] = time.Unix(second, 0).In(original[i].Location())
	}
	return result, fellBack, nil
}

// randomSeconds fills seconds with random dates from first to first+span,
// at least opts.MinGap apart. The minimum gaps are set aside first, the
// commits in between are placed at random in the time that is left, and each
// one is then pushed later by the gaps before it. That keeps them in order and
// apart without retrying, and spreads them uniformly over the time that is
// left.
func randomSeconds(seconds []int64, first, span int64, opts SpreadOptions) error {
	count := len(seconds)
	minGap := int64(opts.MinGap / time.Second)
	if minGap < 0 {
		return fmt.Errorf("%w: the minimum gap cannot be negative", ErrInvalidSpreadRange)
	}
	free := span - int64(count-1)*minGap
	if free < 0 {
		return fmt.Errorf("%w: %d commits at least %s apart need at least %s between the first and last date",
			ErrInvalidSpreadRange, count, formatSpan(minGap), formatSpan(int64(count-1)*minGap))
	}

	random := rand.New(rand.NewPCG(opts.Seed, opts.Seed))
	inner := make([]int64, count-2)
	for i := range inner {
		inner[i] = random.Int64N(free + 1)
	}
	slices.Sort(inner)
	for i, position := range inner {
		seconds[i+1] = first + position + int64(i+1)*minGap
	}
	return nil
}

// formatSpan formats seconds for error messages, e.g. "1d 4h 40m" or "30s".
func formatSpan(seconds int64) string {
	units := []struct {
		size int64
		unit string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}, {1, "s"}}
	var parts []string
	for _, u := range units {
		if seconds >= u.size {
			parts = append(parts, fmt.Sprintf("%d%s", seconds/u.size, u.unit))
			seconds %= u.size
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// inHistoryOrder reports whether dates (oldest commit first) never go back
// in time and do not all fall on the same second, so their gaps can be
// scaled.
func inHistoryOrder(dates []time.Time) bool {
	for i := 1; i < len(dates); i++ {
		if dates[i].Unix() < dates[i-1].Unix() {
			return false
		}
	}
	return dates[len(dates)-1].Unix() > dates[0].Unix()
}
