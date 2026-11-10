// Package overdue is the one rule for when an Invoice is late and by how
// many days (#1965, ADR-0038). It imports nothing of this repo's own but
// the leaf invoicestatus, so every read that shows an Invoice can count it
// here: the payments package's Invoice lists and the Clients list's rollup
// (#2130), which payments itself imports and so cannot import back. One
// rule in one place means one Invoice reads Overdue on every Staff screen
// or on none.
//
// An Invoice with a payment on its way (#2225) is never late: the Client
// has paid and the money is clearing, so calling it overdue would ask the
// Practice to chase money already sent. Days and Expr both hold that rule,
// so a row's count, the Practice's overdue total and its ?overdue=true
// list agree with the "Payment on its way" every Staff screen labels it
// with.
package overdue

import (
	"time"

	"doula-cloud/api/internal/invoicestatus"
)

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
// written-off Invoices are never late, and neither is one with a payment
// on its way. The first overdue day counts as one. Counted in calendar
// days of the Practice's zone, so a 23- or 25-hour day across a clock
// change still counts as one. Expr is the same rule in SQL.
func Days(status string, paymentOnItsWay bool, dueAt, now time.Time, loc *time.Location) int {
	if invoicestatus.Status(status) != invoicestatus.Open || paymentOnItsWay || !dueAt.Before(From(now, loc)) {
		return 0
	}
	return int(civilDate(now, loc).Sub(civilDate(dueAt, loc)) / (24 * time.Hour))
}

// Expr is Days > 0 as a SQL predicate over an invoices row aliased i:
// open, nothing on its way, and due before cutoffParam, the query
// parameter that carries From's instant. The Practice's overdue totals
// and its ?overdue=true narrowing both read it, so neither can count an
// Invoice the row itself does not call late.
func Expr(cutoffParam string) string {
	return `(i.status = 'open' AND NOT ` + invoicestatus.PaymentOnItsWayExpr + ` AND i.due_at < ` + cutoffParam + `)`
}

// civilDate is at's calendar day in loc, as UTC midnight of that day, so
// two of them subtract to a whole number of 24-hour days.
func civilDate(at time.Time, loc *time.Location) time.Time {
	year, month, day := at.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
