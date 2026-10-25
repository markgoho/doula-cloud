package payments

import (
	"database/sql"
	"time"
)

// An Invoice carries two instants a person reads as calendar days: when
// it falls due (due_at) and when the money came in (paid_at). #1965 sends
// both as date-only YYYY-MM-DD fields, read in the Practice's own zone
// (practices.timezone, ADR-0036), because a day only exists somewhere and
// the Practice's book is kept in one place, not wherever its reader is.
// The stored instants do not change: the audit trail keeps the moment a
// Payment was received, and Stripe is still told the due instant.
//
// The zone is loaded once per request by practicetimezone.Load and
// passed in, the same seam visit.DeriveType and the manual Payment's
// future-date guard spend. Nothing here converts a zone in SQL.

// practiceDay is the calendar day at falls on in loc, in paidOnLayout
// form.
func practiceDay(at time.Time, loc *time.Location) string {
	return at.In(loc).Format(paidOnLayout)
}

// invoicePaidOn is the day an Invoice was paid, or nil when it is not
// paid. paid_at has two writers, and they mean different things by it:
//
//   - The invoice.paid webhook stores the instant Stripe collected the
//     card Payment. Its day is the day that instant falls on in the
//     Practice's zone, so a Payment at 8pm in New York on Oct 25 is paid
//     on Oct 25 even though it is Oct 26 in UTC.
//   - PostManualPaymentHandler stores the day the recorder typed as UTC
//     midnight of that day. The day is the one she typed, in any zone;
//     reading it in New York would move it back one.
//
// recordedByHand says which writer it was: the covering Payment's kind is
// 'manual'. An Invoice paid with no Payment row behind it is not one this
// package writes, and is read as an instant.
func invoicePaidOn(paidAt sql.NullTime, recordedByHand bool, loc *time.Location) *string {
	if !paidAt.Valid {
		return nil
	}
	day := practiceDay(paidAt.Time, loc)
	if recordedByHand {
		day = paidAt.Time.UTC().Format(paidOnLayout)
	}
	return &day
}

// overdueFrom is the instant an Invoice due before it is overdue at now:
// midnight starting the Practice's own today. An Invoice is overdue from
// the start of the day after its due day in the Practice's zone (#1965),
// and "its due day is before today" is the same test as "due_at is
// before midnight starting today", which is a plain instant comparison
// Postgres can make against the stored column. Read once per request, so
// the list, its narrowing and the totals above it cannot disagree.
//
// Midnight exists on every day in every US zone, whose clocks change at
// 2am. In a zone whose clock jumps at midnight, time.Date normalizes the
// missing instant to the first one that exists, which is still the start
// of that day.
func overdueFrom(now time.Time, loc *time.Location) time.Time {
	year, month, day := now.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// daysOverdue is how many whole Practice days an open Invoice is past its
// due day at now, and 0 when it is not overdue or not open: paid, voided
// and written-off Invoices are never late. The first overdue day counts
// as one. Counted in calendar days of the Practice's zone, so a 23- or
// 25-hour day across a clock change still counts as one.
func daysOverdue(status string, dueAt, now time.Time, loc *time.Location) int {
	if status != invoiceStatusOpen || !dueAt.Before(overdueFrom(now, loc)) {
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
