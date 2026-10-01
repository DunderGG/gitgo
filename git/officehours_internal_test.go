package git

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// officeWeek is Monday to Friday, 09:00 to 17:00.
var officeWeek = OfficeHours{
	Start: 9 * time.Hour,
	End:   17 * time.Hour,
	Days:  [7]bool{time.Monday: true, time.Tuesday: true, time.Wednesday: true, time.Thursday: true, time.Friday: true},
}

// monday returns Monday 5 February 2024 at hour:minute in UTC, plus days.
func monday(days, hour, minute int) time.Time {
	return time.Date(2024, 2, 5+days, hour, minute, 0, 0, time.UTC)
}

// sameDates returns n copies of spreadBase, as original dates that carry no
// pattern.
func sameDates(n int) []time.Time {
	dates := make([]time.Time, n)
	for i := range dates {
		dates[i] = spreadBase
	}
	return dates
}

func assertDates(test *testing.T, got []time.Time, want ...time.Time) {
	test.Helper()
	if len(got) != len(want) {
		test.Fatalf("got %d dates, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			test.Errorf("date %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestSpreadTimes_OfficeHoursSkipNights verifies that even spacing counts
// only office time, so the commits continue the next morning.
func TestSpreadTimes_OfficeHoursSkipNights(test *testing.T) {
	hours := officeWeek
	opts := SpreadOptions{First: monday(0, 16, 0), Last: monday(1, 10, 0), Spacing: EvenSpacing, OfficeHours: &hours}

	got, _, err := spreadTimes(sameDates(5), opts)
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	// Two hours of office time, so 30 minutes apart.
	assertDates(test, got, monday(0, 16, 0), monday(0, 16, 30), monday(0, 17, 0), monday(1, 9, 30), monday(1, 10, 0))
}

// TestSpreadTimes_OfficeHoursSkipWeekends verifies that non-working days
// count as no time at all.
func TestSpreadTimes_OfficeHoursSkipWeekends(test *testing.T) {
	hours := officeWeek
	opts := SpreadOptions{First: monday(4, 16, 0), Last: monday(7, 10, 0), Spacing: EvenSpacing, OfficeHours: &hours}

	got, _, err := spreadTimes(sameDates(5), opts)
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	// Friday 16:00 to Monday 10:00 is two hours of office time.
	assertDates(test, got, monday(4, 16, 0), monday(4, 16, 30), monday(4, 17, 0), monday(7, 9, 30), monday(7, 10, 0))
}

// TestSpreadTimes_OfficeHoursKeepRelativeSpacing verifies that the current
// pattern is scaled onto office time.
func TestSpreadTimes_OfficeHoursKeepRelativeSpacing(test *testing.T) {
	hours := officeWeek
	original := []time.Time{at(0), at(10 * time.Minute), at(time.Hour)}
	opts := SpreadOptions{First: monday(0, 16, 0), Last: monday(1, 11, 0), OfficeHours: &hours}

	got, fellBack, err := spreadTimes(original, opts)
	if err != nil || fellBack {
		test.Fatalf("spreadTimes: fellBack=%v, err=%v", fellBack, err)
	}
	// Three hours of office time: a sixth of it is 30 minutes.
	assertDates(test, got, monday(0, 16, 0), monday(0, 16, 30), monday(1, 11, 0))
}

// TestSpreadTimes_OfficeHoursRandomStaysInside verifies that random spacing
// never puts a commit outside office hours, and measures the minimum gap in
// office time.
func TestSpreadTimes_OfficeHoursRandomStaysInside(test *testing.T) {
	hours := officeWeek
	minGap := 2 * time.Hour
	for seed := uint64(0); seed < 200; seed++ {
		opts := SpreadOptions{
			First: monday(0, 9, 0), Last: monday(9, 17, 0),
			Spacing: RandomSpacing, MinGap: minGap, Seed: seed, OfficeHours: &hours,
		}
		got, _, err := spreadTimes(sameDates(12), opts)
		if err != nil {
			test.Fatalf("seed %d: %v", seed, err)
		}
		for i, date := range got {
			if !hours.contains(date) {
				test.Fatalf("seed %d: date %d (%v, %v) is outside office hours", seed, i, date, date.Weekday())
			}
			if i == 0 {
				continue
			}
			between, err := newTimeAxis(got[i-1], date, &hours, true)
			if err != nil || between.span < int64(minGap/time.Second) {
				test.Fatalf("seed %d: %v to %v is %ds of office time (%v), under %v",
					seed, got[i-1], date, between.span, err, minGap)
			}
		}
	}
}

// TestSpreadTimes_OfficeHoursUseFirstLocation verifies that the office hours
// are read in the first date's time zone, while each commit keeps its own.
func TestSpreadTimes_OfficeHoursUseFirstLocation(test *testing.T) {
	hours := officeWeek
	stockholm := time.FixedZone("", 3600)
	opts := SpreadOptions{
		First: time.Date(2024, 2, 5, 9, 0, 0, 0, stockholm), Last: time.Date(2024, 2, 5, 17, 0, 0, 0, stockholm),
		Spacing: EvenSpacing, OfficeHours: &hours,
	}

	got, _, err := spreadTimes(sameDates(3), opts)
	if err != nil {
		test.Fatalf("spreadTimes: %v", err)
	}
	want := []string{"2024-02-05T08:00:00Z", "2024-02-05T12:00:00Z", "2024-02-05T16:00:00Z"}
	for i, date := range got {
		if text := date.Format(time.RFC3339); text != want[i] {
			test.Errorf("date %d = %s, want %s", i, text, want[i])
		}
	}
}

// TestSpreadTimes_OfficeHoursRejectEndsOutside verifies that a first or last
// date outside office hours is refused rather than moved, also for a single
// commit, and that a range too short in office time says so.
func TestSpreadTimes_OfficeHoursRejectEndsOutside(test *testing.T) {
	hours := officeWeek
	cases := map[string]struct {
		count       int
		first, last time.Time
		wantText    string
	}{
		"first before opening":   {3, monday(0, 8, 59), monday(0, 12, 0), "first date is outside"},
		"last after closing":     {3, monday(0, 9, 0), monday(0, 17, 1), "last date is outside"},
		"first on a sunday":      {3, monday(-1, 12, 0), monday(0, 12, 0), "first date is outside"},
		"single commit at night": {1, monday(0, 22, 0), monday(0, 22, 0), "first date is outside"},
		"too little office time": {5, monday(0, 16, 59), monday(1, 9, 1), "of office hours"},
	}
	for name, c := range cases {
		opts := SpreadOptions{First: c.first, Last: c.last, Spacing: RandomSpacing, MinGap: time.Minute, OfficeHours: &hours}
		_, _, err := spreadTimes(sameDates(c.count), opts)
		if !errors.Is(err, ErrInvalidSpreadRange) || !strings.Contains(err.Error(), c.wantText) {
			test.Errorf("%s: err = %v, want ErrInvalidSpreadRange mentioning %q", name, err, c.wantText)
		}
	}
}

// TestOfficeHours_Validate verifies that office hours must open before they
// close and have a working day.
func TestOfficeHours_Validate(test *testing.T) {
	if err := officeWeek.Validate(); err != nil {
		test.Fatalf("Validate(Mon–Fri 09:00–17:00) = %v", err)
	}
	invalid := map[string]OfficeHours{
		"closes before opening": {Start: 17 * time.Hour, End: 9 * time.Hour, Days: officeWeek.Days},
		"closes when opening":   {Start: 9 * time.Hour, End: 9 * time.Hour, Days: officeWeek.Days},
		"past midnight":         {Start: 22 * time.Hour, End: 26 * time.Hour, Days: officeWeek.Days},
		"no working days":       {Start: 9 * time.Hour, End: 17 * time.Hour},
	}
	for name, hours := range invalid {
		if err := hours.Validate(); err == nil {
			test.Errorf("%s: Validate accepted %+v", name, hours)
		}
	}
}
