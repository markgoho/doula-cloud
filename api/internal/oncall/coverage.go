package oncall

import (
	"slices"
	"time"
)

// UnstaffedDays is every run of days inside window on which no Doula is on
// call -- the holes narrowings leave -- that meets rng. A run is whole:
// it is not cut at the range's edges, so the same hole reads the same
// under any range that reaches it. effectives is each
// attached Doula's own effective interval (Window.Effective), in any
// order and possibly overlapping. A window with nobody's narrowing
// leaving a gap returns nil.
//
// Day-granular because narrowings are: a coverage gap, which is hours,
// is reported beside this rather than folded into it, so the roster can
// say "nobody is on call on the 16th" and "Maya cannot be reached on
// Saturday night, and nobody is covering" as the two different facts
// they are.
func UnstaffedDays(window, rng Window, effectives []Window) []Window {
	span := window
	if !span.Overlaps(rng) {
		return nil
	}

	sorted := slices.Clone(effectives)
	slices.SortFunc(sorted, func(a, b Window) int {
		switch {
		case a.Start < b.Start:
			return -1
		case a.Start > b.Start:
			return 1
		}
		return 0
	})

	var holes []Window
	cursor := span.Start
	for _, e := range sorted {
		if e.End < cursor {
			continue
		}
		if e.Start > cursor {
			holes = appendHole(holes, cursor, dayBefore(e.Start), span.End)
		}
		if e.End >= span.End {
			return meeting(holes, rng)
		}
		// e.End >= cursor here, so the day after it is always later.
		cursor = dayAfter(e.End)
	}
	return meeting(appendHole(holes, cursor, span.End, span.End), rng)
}

func meeting(holes []Window, rng Window) []Window {
	var out []Window
	for _, h := range holes {
		if h.Overlaps(rng) {
			out = append(out, h)
		}
	}
	return out
}

// appendHole adds [from, to] clipped to limit. from is never past limit:
// the cursor only ever moves to the day after an interval that ended
// before limit, so there is always at least one day left.
func appendHole(holes []Window, from, to, limit string) []Window {
	if to > limit {
		to = limit
	}
	return append(holes, Window{Start: from, End: to})
}

func dayAfter(day string) string  { return shiftDay(day, 1) }
func dayBefore(day string) string { return shiftDay(day, -1) }

func shiftDay(day string, by int) string {
	t, _ := time.Parse(dateLayout, day)
	return t.AddDate(0, 0, by).Format(dateLayout)
}
