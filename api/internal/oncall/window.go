// Package oncall answers the question an agency owner asks every
// evening (#1093): who is on call tonight, and is anyone covering two
// births at once.
//
// Three things live here. The on-call window, which is derived and never
// stored (this file). The coverage gap, which is a stated interval inside
// a window when one attached Doula cannot be reached (gap.go). And the
// roster read that puts both in front of a Practice, which is where the
// contractor refusal lives (roster.go).
//
// None of it is a new reach mechanism. Everything here reads through the
// Attachment, exactly as ADR-0006 and ADR-0008 already draw it.
package oncall

import "time"

// dateLayout is the plain YYYY-MM-DD shape every date column in the BFF
// is read and compared as (visit.dateLayout, engagement.dateLayout).
const dateLayout = "2006-01-02"

// fullTermWeeks is the gestational age a due date names: 40w0d. A
// gestational-week start rule counts back from the due date by the
// difference.
const fullTermWeeks = 40

// StartRule is the on_call_start_rule enum (00114).
type StartRule string

// The two start rules a Practice chooses between, and an Engagement may
// override.
const (
	// StartGestationalWeek opens the window at a gestational week --
	// typically 37w0d -- counted back from the due date.
	StartGestationalWeek StartRule = "gestational_week"
	// StartAttachmentGranted opens the window on the day the first open,
	// granted Attachment on the Engagement was made: "I'm on call 24/7
	// from time of hire".
	StartAttachmentGranted StartRule = "attachment_granted"
)

// Rule is what decides one Engagement's window: how it starts, and how
// long it runs past the due date where no pregnancy end is recorded.
type Rule struct {
	Start     StartRule
	Week      int
	GraceDays int
}

// Resolve applies an Engagement's own override, if it carries one, over
// the Practice's rule. Only the start is overridable; the grace stays
// the Practice's, because the ticket names the start and nothing else as
// a per-Engagement choice. rule and week are the two nullable engagements
// columns exactly as read.
func (r Rule) Resolve(rule *string, week *int16) Rule {
	if rule == nil {
		return r
	}
	out := r
	out.Start = StartRule(*rule)
	if week != nil {
		out.Week = int(*week)
	}
	return out
}

// NoWindowReason says why an Engagement has no window, so the roster can
// state it rather than guess a date.
type NoWindowReason string

// The reasons DeriveWindow can give. The empty string means there is a
// window.
const (
	// NoWindowNoDueDate: neither a start nor an end can be resolved
	// without a due date. Under a gestational rule the start needs it;
	// under either rule the end needs it until the pregnancy has ended.
	NoWindowNoDueDate NoWindowReason = "no_due_date"
	// NoWindowPostpartum: postpartum work is shift-shaped, not
	// on-call-shaped, so a postpartum Engagement never has a window.
	NoWindowPostpartum NoWindowReason = "postpartum"
	// NoWindowNotActive: care has not started, or has ended.
	NoWindowNotActive NoWindowReason = "not_active"
	// NoWindowNobodyAttached: nobody holds a granted Attachment, so
	// nobody can be on call for it.
	NoWindowNobodyAttached NoWindowReason = "nobody_attached"
	// NoWindowEndedBeforeStart: the pregnancy ended before the window
	// would have opened -- an early birth under a gestational rule.
	NoWindowEndedBeforeStart NoWindowReason = "ended_before_start"
)

// WindowInput is every fact one Engagement's window is derived from, as
// read off its row. Dates are YYYY-MM-DD text straight off `date`
// columns; FirstGrantedOn is the earliest open, granted Attachment's
// attached_at, already turned into a calendar day in the Practice's zone
// by the caller.
type WindowInput struct {
	Kind             string
	Status           string
	DueDate          *string
	PregnancyEndedOn *string
	FirstGrantedOn   *string
	Rule             Rule
}

// Window is an on-call window: two calendar days in the Practice's zone,
// both inclusive.
type Window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// DeriveWindow is the one place an on-call window is computed, and every
// reader calls it -- the same shape as visit.DeriveType. Nothing stores a
// window, so correcting a due date or a pregnancy-end date moves it on
// the next read, with no backfill and no write to any other row.
//
// Start is the rule's: a gestational week counted back from the due
// date, or the day the Attachment was granted. End is the recorded
// pregnancy end where there is one -- the Doula is on call through the
// birth day -- and otherwise the due date plus the Practice's grace.
func DeriveWindow(in WindowInput) (*Window, NoWindowReason) {
	switch {
	case in.Kind != "birth":
		return nil, NoWindowPostpartum
	case in.Status != "active":
		return nil, NoWindowNotActive
	case isBlank(in.FirstGrantedOn):
		return nil, NoWindowNobodyAttached
	}

	due, hasDue := parseDate(in.DueDate)
	ended, hasEnded := parseDate(in.PregnancyEndedOn)

	var start time.Time
	switch in.Rule.Start {
	case StartAttachmentGranted:
		start, _ = parseDate(in.FirstGrantedOn)
	default:
		if !hasDue {
			return nil, NoWindowNoDueDate
		}
		start = due.AddDate(0, 0, -7*(fullTermWeeks-in.Rule.Week))
	}

	var end time.Time
	switch {
	case hasEnded:
		end = ended
	case hasDue:
		end = due.AddDate(0, 0, in.Rule.GraceDays)
	default:
		return nil, NoWindowNoDueDate
	}

	if end.Before(start) {
		return nil, NoWindowEndedBeforeStart
	}
	return &Window{Start: start.Format(dateLayout), End: end.Format(dateLayout)}, ""
}

// Effective is the part of w one Doula is on call for, given her
// narrowing: the window itself where she has none, and the overlap of
// the two where she does. An open bound on the narrowing runs to the
// window's own edge. ok is false where the narrowing and the window do
// not meet at all -- she is attached, and on call for none of it.
//
// Every value is YYYY-MM-DD, which orders as text, so no parse is needed.
func (w Window) Effective(from, to *string) (Window, bool) {
	out := w
	if !isBlank(from) && *from > out.Start {
		out.Start = *from
	}
	if !isBlank(to) && *to < out.End {
		out.End = *to
	}
	if out.End < out.Start {
		return Window{}, false
	}
	return out, true
}

// Bounds turns w's two calendar days into the half-open instant range
// they cover in zone: midnight at the start of the first day, to
// midnight at the start of the day after the last.
func (w Window) Bounds(zone *time.Location) (from, to time.Time) {
	return DayStart(w.Start, zone), DayStart(w.End, zone).AddDate(0, 0, 1)
}

// DayStart is midnight at the start of day in zone. day is YYYY-MM-DD
// and always one this package produced or validated; an unparseable one
// is a programming error and reads as the zero time.
func DayStart(day string, zone *time.Location) time.Time {
	t, err := time.ParseInLocation(dateLayout, day, zone)
	if err != nil {
		// coverage:ignore reason: every caller passes a day this package produced or already validated
		return time.Time{}
	}
	return t
}

// parseDate reads a YYYY-MM-DD column value as a UTC civil date. UTC here
// carries no meaning: it is only the zone the day arithmetic runs in, and
// AddDate on a UTC date never meets a daylight-saving jump.
func parseDate(s *string) (time.Time, bool) {
	if isBlank(s) {
		return time.Time{}, false
	}
	t, err := time.Parse(dateLayout, *s)
	if err != nil {
		// coverage:ignore reason: every caller passes a value read off a Postgres date column
		return time.Time{}, false
	}
	return t, true
}

func isBlank(s *string) bool {
	return s == nil || *s == ""
}

// Overlaps reports whether w and other share at least one day.
func (w Window) Overlaps(other Window) bool {
	return w.Start <= other.End && other.Start <= w.End
}
