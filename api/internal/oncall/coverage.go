package oncall

import (
	"slices"
	"time"
)

// UnstaffedDays is every run of days inside both window and rng on which
// no Doula is on call -- the holes narrowings leave. effectives is each
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
	span, ok := window.Effective(&rng.Start, &rng.End)
	if !ok {
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
			return holes
		}
		cursor = maxDay(cursor, dayAfter(e.End))
	}
	return appendHole(holes, cursor, span.End, span.End)
}

// appendHole adds [from, to] clipped to limit, if anything is left.
func appendHole(holes []Window, from, to, limit string) []Window {
	if to > limit {
		to = limit
	}
	if from > to {
		return holes
	}
	return append(holes, Window{Start: from, End: to})
}

func dayAfter(day string) string  { return shiftDay(day, 1) }
func dayBefore(day string) string { return shiftDay(day, -1) }

func shiftDay(day string, by int) string {
	t, _ := time.Parse(dateLayout, day)
	return t.AddDate(0, 0, by).Format(dateLayout)
}

func maxDay(a, b string) string {
	if a > b {
		return a
	}
	return b
}
