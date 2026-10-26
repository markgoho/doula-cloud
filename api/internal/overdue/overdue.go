// Package overdue is the one rule for when an Invoice is late and by how
// many days (#1965, ADR-0038). It imports nothing of this repo's own, so
// every read that shows an Invoice can count it here: the payments
// package's Invoice lists and the Clients list's rollup (#2130), which
// payments itself imports and so cannot import back. One rule in one
// place means one Invoice reads Overdue on every Staff screen or on none.
package overdue

import "time"

// statusOpen is the stored invoice_status of a billed, unpaid Invoice:
// the only status that can be late.
const statusOpen = "open"

// From is the instant an Invoice due before it is overdue at now:
// midnight starting the Practice's own today. An Invoice is overdue from
// the start of the day after its due day in the Practice's zone (#1965),
// and "its due day is before today" is the same test as "due_at is
// before midnight starting today", which is a plain instant comparison
// Postgres can make against the stored column. Read once per request, so
// a list, its narrowing and the totals above it cannot disagree.
//
// Midnight exists on every day in every US zone, whose clocks change at
// 2am. In a zone whose clock jumps at midnight, time.Date normalizes the
// missing midnight to an instant within an hour of it; which one is not
// guaranteed, so there the boundary can be off by that hour.
func From(now time.Time, loc *time.Location) time.Time {
	year, month, day := now.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// Days is how many whole Practice days an open Invoice is past its due
// day at now, and 0 when it is not overdue or not open: paid, voided and
// written-off Invoices are never late. The first overdue day counts as
// one. Counted in calendar days of the Practice's zone, so a 23- or
// 25-hour day across a clock change still counts as one.
func Days(status string, dueAt, now time.Time, loc *time.Location) int {
	if status != statusOpen || !dueAt.Before(From(now, loc)) {
		return 0
	}
	return int(civilDate(now, loc).Sub(civilDate(dueAt, loc)) / (24 * time.Hour))
}

// civilDate is at's calendar day in loc, as UTC midnight of that day, so
// two of them subtract to a whole number of 24-hour days.
func civilDate(at time.Time, loc *time.Location) time.Time {
	year, month, day := at.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
