package git

import (
	"fmt"
	"time"
)

// OfficeHours is a daily window of working time on chosen weekdays, used to
// keep spread commits out of nights and weekends (see SpreadOptions).
type OfficeHours struct {
	// Start and End are the times of day the window opens and closes, as
	// the time since midnight. End is after Start, on the same day.
	Start time.Duration
	End   time.Duration
	// Days are the working days, indexed by time.Weekday.
	Days [7]bool
}

// Validate returns an error unless the window opens before it closes, within
// one day, on at least one day of the week.
func (hours OfficeHours) Validate() error {
	if hours.Start < 0 || hours.End > 24*time.Hour || hours.Start >= hours.End {
		return fmt.Errorf("office hours must start before they end, within one day")
	}
	for _, working := range hours.Days {
		if working {
			return nil
		}
	}
	return fmt.Errorf("office hours need at least one working day")
}

// window returns when the office opens and closes on the day of t, in t's
// location, and whether that day is a working day.
func (hours OfficeHours) window(t time.Time) (opens, closes time.Time, working bool) {
	midnight := midnightOf(t)
	return clockTime(midnight, hours.Start), clockTime(midnight, hours.End), hours.Days[t.Weekday()]
}

// midnightOf is the start of t's date, in t's location.
func midnightOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// contains reports whether t falls within office hours, both ends included.
func (hours OfficeHours) contains(t time.Time) bool {
	opens, closes, working := hours.window(t)
	return working && !t.Before(opens) && !t.After(closes)
}

// clockTime is the time of day offset after midnight on the same date. It
// uses the wall clock rather than adding a duration, so a daylight saving
// change during the night does not move the office hours.
func clockTime(midnight time.Time, offset time.Duration) time.Time {
	minutes := int(offset / time.Minute)
	return time.Date(midnight.Year(), midnight.Month(), midnight.Day(), minutes/60, minutes%60, 0, 0, midnight.Location())
}

// timeAxis is the timeline that commits are spread on: whole seconds from
// the first date to the last, leaving out the time outside office hours when
// there are office hours. Position 0 is the first date and span the last.
type timeAxis struct {
	// segments are the stretches of usable time, in order, as Unix seconds.
	segments []axisSegment
	span     int64
	// unit describes the timeline in error messages, e.g. "between the
	// first and last date".
	unit string
}

type axisSegment struct {
	start, length int64
}

// newTimeAxis builds the timeline from first to last. With office hours, it
// keeps only their time, in first's location, and first (and, when several
// is set, last) must be within them. With several set, last must be after
// first.
func newTimeAxis(first, last time.Time, hours *OfficeHours, several bool) (timeAxis, error) {
	if several && last.Unix() <= first.Unix() {
		return timeAxis{}, fmt.Errorf("%w: the last date must be after the first", ErrInvalidSpreadRange)
	}
	if hours == nil {
		span := max(last.Unix()-first.Unix(), 0)
		return timeAxis{
			segments: []axisSegment{{start: first.Unix(), length: span}},
			span:     span,
			unit:     "between the first and last date",
		}, nil
	}

	if err := hours.Validate(); err != nil {
		return timeAxis{}, err
	}
	last = last.In(first.Location())
	if !hours.contains(first) {
		return timeAxis{}, fmt.Errorf("%w: the first date is outside office hours", ErrInvalidSpreadRange)
	}
	if several && !hours.contains(last) {
		return timeAxis{}, fmt.Errorf("%w: the last date is outside office hours", ErrInvalidSpreadRange)
	}
	axis := timeAxis{unit: "of office hours between the first and last date"}
	if !several {
		axis.segments = []axisSegment{{start: first.Unix()}}
		return axis, nil
	}

	// One segment per working day, cut to the range. A working day can be
	// cut to nothing at the ends, for example a first date at closing time;
	// it is kept so that position 0 is still the first date.
	lastDay := midnightOf(last)
	for day := midnightOf(first); !day.After(lastDay); day = day.AddDate(0, 0, 1) {
		opens, closes, working := hours.window(day)
		if !working {
			continue
		}
		start, end := max(opens.Unix(), first.Unix()), min(closes.Unix(), last.Unix())
		if start <= end {
			axis.segments = append(axis.segments, axisSegment{start: start, length: end - start})
			axis.span += end - start
		}
	}
	return axis, nil
}

// at returns the date at position on the timeline. A position where one
// segment ends and the next begins is the end of the earlier one, so a
// commit can land at closing time but never jumps over a night it does not
// need.
func (axis timeAxis) at(position int64) time.Time {
	for _, segment := range axis.segments {
		if position <= segment.length {
			return time.Unix(segment.start+position, 0)
		}
		position -= segment.length
	}
	last := axis.segments[len(axis.segments)-1]
	return time.Unix(last.start+last.length, 0)
}
