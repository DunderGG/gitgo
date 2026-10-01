package git

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

var spreadBase = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// at returns spreadBase plus d, for listing original dates.
func at(d time.Duration) time.Time { return spreadBase.Add(d) }

// offsetsFrom returns how far each date is from first, for comparing results.
func offsetsFrom(first time.Time, dates []time.Time) []time.Duration {
	result := make([]time.Duration, len(dates))
	for i, date := range dates {
		result[i] = date.Sub(first)
	}
	return result
}

func assertOffsets(test *testing.T, got, want []time.Duration) {
	test.Helper()
	if len(got) != len(want) {
		test.Fatalf("got %d dates, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			test.Errorf("date %d is %v after the first, want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

// TestSpreadTimes_KeepsRelativeSpacing verifies that the original gaps are
// scaled to the new range.
func TestSpreadTimes_KeepsRelativeSpacing(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(10 * time.Minute), at(time.Hour)}

	got, fellBack, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(2 * time.Hour)})
	if err != nil || fellBack {
		test.Fatalf("spreadTimes: fellBack=%v, err=%v", fellBack, err)
	}
	assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, 20 * time.Minute, 2 * time.Hour})
}

// TestSpreadTimes_EvenSpacing verifies equal gaps regardless of the original
// dates.
func TestSpreadTimes_EvenSpacing(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(time.Minute), at(2 * time.Minute), at(10 * time.Hour)}

	got, fellBack, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(3 * time.Hour), Spacing: EvenSpacing})
	if err != nil || fellBack {
		test.Fatalf("spreadTimes: fellBack=%v, err=%v", fellBack, err)
	}
	assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, time.Hour, 2 * time.Hour, 3 * time.Hour})
}

// TestSpreadTimes_FallsBackToEvenSpacing verifies that relative spacing is
// replaced by even spacing, and reported, when the original dates are all
// equal or go back in time.
func TestSpreadTimes_FallsBackToEvenSpacing(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	cases := map[string][]time.Time{
		"all equal":    {at(0), at(0), at(0)},
		"out of order": {at(time.Hour), at(0), at(2 * time.Hour)},
	}
	for name, original := range cases {
		got, fellBack, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(2 * time.Hour)})
		if err != nil || !fellBack {
			test.Fatalf("%s: fellBack=%v, err=%v, want a fallback", name, fellBack, err)
		}
		assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, time.Hour, 2 * time.Hour})
	}
}

// TestSpreadTimes_KeepsEqualNeighbours verifies that commits sharing a date
// keep sharing one with relative spacing, without a fallback.
func TestSpreadTimes_KeepsEqualNeighbours(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(0), at(time.Hour)}

	got, fellBack, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(4 * time.Hour)})
	if err != nil || fellBack {
		test.Fatalf("spreadTimes: fellBack=%v, err=%v", fellBack, err)
	}
	assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, 0, 4 * time.Hour})
}

// TestSpreadTimes_RoundsToWholeSeconds verifies that gaps which do not divide
// evenly are rounded to the nearest second.
func TestSpreadTimes_RoundsToWholeSeconds(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(0), at(0), at(0)}

	got, _, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(10 * time.Second), Spacing: EvenSpacing})
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, 3 * time.Second, 7 * time.Second, 10 * time.Second})
}

// TestSpreadTimes_RandomKeepsMinimumGapAndEnds verifies that random spacing
// keeps the first and last dates exact, never puts two commits closer than
// the minimum gap, actually varies with the seed, and is repeatable with the
// same seed.
func TestSpreadTimes_RandomKeepsMinimumGapAndEnds(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	last := first.Add(5 * time.Hour)
	original := make([]time.Time, 8)
	for i := range original {
		original[i] = at(0)
	}
	minGap := 20 * time.Minute

	layouts := make(map[string]bool)
	for seed := uint64(0); seed < 200; seed++ {
		opts := SpreadOptions{First: first, Last: last, Spacing: RandomSpacing, MinGap: minGap, Seed: seed}
		got, fellBack, err := spreadTimes(original, opts)
		if err != nil || fellBack {
			test.Fatalf("seed %d: fellBack=%v, err=%v", seed, fellBack, err)
		}
		if !got[0].Equal(first) || !got[len(got)-1].Equal(last) {
			test.Fatalf("seed %d: ends are %v and %v, want %v and %v", seed, got[0], got[len(got)-1], first, last)
		}
		for i := 1; i < len(got); i++ {
			if gap := got[i].Sub(got[i-1]); gap < minGap {
				test.Fatalf("seed %d: gap %d is %v, under the minimum %v", seed, i, gap, minGap)
			}
		}
		layouts[fmt.Sprint(offsetsFrom(first, got))] = true

		again, _, _ := spreadTimes(original, opts)
		for i := range got {
			if !again[i].Equal(got[i]) {
				test.Fatalf("seed %d: date %d differs between runs: %v and %v", seed, i, got[i], again[i])
			}
		}
	}
	if len(layouts) < 190 {
		test.Errorf("200 seeds gave only %d different layouts", len(layouts))
	}
}

// TestSpreadTimes_RandomFillsExactlyFittingRange verifies that a range of
// exactly the minimum gaps leaves no room for chance: the commits are evenly
// spaced at the minimum gap.
func TestSpreadTimes_RandomFillsExactlyFittingRange(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(0), at(0), at(0)}

	opts := SpreadOptions{First: first, Last: first.Add(30 * time.Minute), Spacing: RandomSpacing, MinGap: 10 * time.Minute, Seed: 7}
	got, _, err := spreadTimes(original, opts)
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	assertOffsets(test, offsetsFrom(first, got), []time.Duration{0, 10 * time.Minute, 20 * time.Minute, 30 * time.Minute})
}

// TestSpreadTimes_KeepsEachOffset verifies that every commit keeps its own
// time zone offset while the moments come from the range.
func TestSpreadTimes_KeepsEachOffset(test *testing.T) {
	india := time.FixedZone("", 5*3600+1800)
	brazil := time.FixedZone("", -3*3600)
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0).In(india), at(time.Hour).In(brazil)}

	got, _, err := spreadTimes(original, SpreadOptions{First: first, Last: first.Add(time.Hour)})
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	want := []string{"2024-02-01T14:30:00+05:30", "2024-02-01T07:00:00-03:00"}
	for i, date := range got {
		if text := date.Format(time.RFC3339); text != want[i] {
			test.Errorf("date %d = %s, want %s", i, text, want[i])
		}
	}
}

// TestSpreadTimes_SingleCommitGetsFirstDate verifies that one commit simply
// moves to the first date, whatever the last date is.
func TestSpreadTimes_SingleCommitGetsFirstDate(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	got, _, err := spreadTimes([]time.Time{at(0)}, SpreadOptions{First: first, Last: first})
	if err != nil || len(got) != 1 || !got[0].Equal(first) {
		test.Fatalf("spreadTimes = %v, %v, want [%v]", got, err, first)
	}
}

// TestSpreadTimes_RejectsInvalidRanges verifies that a last date not after
// the first, even gaps under one second, a range too short for the minimum
// gaps, or a negative minimum gap, are refused.
func TestSpreadTimes_RejectsInvalidRanges(test *testing.T) {
	first := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	original := []time.Time{at(0), at(time.Minute), at(2 * time.Minute)}
	cases := map[string]SpreadOptions{
		"last equals first":            {First: first, Last: first},
		"last before first":            {First: first, Last: first.Add(-time.Hour)},
		"gaps under a second":          {First: first, Last: first.Add(time.Second), Spacing: EvenSpacing},
		"range under the minimum gaps": {First: first, Last: first.Add(19 * time.Minute), Spacing: RandomSpacing, MinGap: 10 * time.Minute},
		"negative minimum gap":         {First: first, Last: first.Add(time.Hour), Spacing: RandomSpacing, MinGap: -time.Minute},
	}
	for name, opts := range cases {
		if _, _, err := spreadTimes(original, opts); !errors.Is(err, ErrInvalidSpreadRange) {
			test.Errorf("%s: err = %v, want ErrInvalidSpreadRange", name, err)
		}
	}
}
