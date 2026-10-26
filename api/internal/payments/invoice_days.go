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

// bookDays is the zone and the instant one read of an Invoice list judges
// every calendar day by (#1965): the Practice-wide list and the
// per-Engagement list, for every reader of it (#2122). Each read builds
// one from practicetimezone.Load and clock.Now(ctx) and counts each row's
// lateness with overdue.Days against it, so no two screens can count one
// Invoice late by different rules or at different instants.
type bookDays struct {
	zone *time.Location
	now  time.Time
}
