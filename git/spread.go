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
// after the first, when the range is too short for the commits, or when the
// first or last date is outside the office hours asked for.
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
	// OfficeHours, when set, keeps the commits within them: the time outside
	// office hours counts as nothing, so every spacing is measured in office
	// time. They are read in First's location, and First and Last must be
	// within them. Nil uses all the time between First and Last.
	OfficeHours *OfficeHours
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
//
// The commits are placed on a timeline of whole seconds from opts.First to
// opts.Last (see timeAxis), which leaves out the time outside office hours
// when opts.OfficeHours is set, and then mapped back to dates.
func spreadTimes(original []time.Time, opts SpreadOptions) ([]time.Time, bool, error) {
	count := len(original)
	axis, err := newTimeAxis(opts.First, opts.Last, opts.OfficeHours, count > 1)
	if err != nil {
		return nil, false, err
	}
	if count == 1 {
		return []time.Time{axis.at(0).In(original[0].Location())}, false, nil
	}

	span := axis.span
	positions := make([]int64, count)
	fellBack := false
	switch {
	case opts.Spacing == KeepSpacing && inHistoryOrder(original):
		// Scale each commit's distance from the oldest one to the new range.
		oldest, originalSpan := original[0].Unix(), float64(original[count-1].Unix()-original[0].Unix())
		for i, date := range original {
			positions[i] = int64(math.Round(float64(date.Unix()-oldest) / originalSpan * float64(span)))
		}
	case opts.Spacing == RandomSpacing:
		if err := randomPositions(positions, span, opts.MinGap, opts.Seed, axis.unit); err != nil {
			return nil, false, err
		}
	default:
		fellBack = opts.Spacing == KeepSpacing
		if span < int64(count-1) {
			return nil, false, fmt.Errorf("%w: %d commits need at least %s %s",
				ErrInvalidSpreadRange, count, formatSpan(int64(count-1)), axis.unit)
		}
		gap := float64(span) / float64(count-1)
		for i := range positions {
			positions[i] = int64(math.Round(float64(i) * gap))
		}
	}
	// The ends are exact even where rounding could have moved them.
	positions[0], positions[count-1] = 0, span

	result := make([]time.Time, count)
	for i, position := range positions {
		result[i] = axis.at(position).In(original[i].Location())
	}
	return result, fellBack, nil
}

// randomPositions fills positions with random points from 0 to span, at
// least minGap apart, with the first at 0 and the last at span. The minimum
// gaps are set aside first, the points in between are placed at random in
// the time that is left, and each one is then pushed later by the gaps
// before it. That keeps them in order and apart without retrying, and
// spreads them uniformly over the time that is left. unit describes span in
// error messages.
func randomPositions(positions []int64, span int64, minGap time.Duration, seed uint64, unit string) error {
	count := len(positions)
	gap := int64(minGap / time.Second)
	if gap < 0 {
		return fmt.Errorf("%w: the minimum gap cannot be negative", ErrInvalidSpreadRange)
	}
	free := span - int64(count-1)*gap
	if free < 0 {
		return fmt.Errorf("%w: %d commits at least %s apart need at least %s %s",
			ErrInvalidSpreadRange, count, formatSpan(gap), formatSpan(int64(count-1)*gap), unit)
	}

	random := rand.New(rand.NewPCG(seed, seed))
	inner := make([]int64, count-2)
	for i := range inner {
		inner[i] = random.Int64N(free + 1)
	}
	slices.Sort(inner)
	for i, position := range inner {
		positions[i+1] = position + int64(i+1)*gap
	}
	positions[0], positions[count-1] = 0, span
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
