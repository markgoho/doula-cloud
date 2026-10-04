package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// DefaultPaymentTermsDays is how many days after an Invoice is raised it
// falls due at a Practice that has never set terms of its own (#768).
//
// 30 is not a new policy: it is the value the Stripe rail already sent as
// days_until_due before this ticket, so adopting it as the default
// changes nothing about an existing Practice's behavior -- it only brings
// the date home, onto the Invoice's own row, where the by-hand rail can
// carry it too. A missing practice_payment_terms row therefore means
// "30 days", not "no due date": unlike practice_rates (#966), where an
// absent row means a rate genuinely nobody has set, every Invoice must
// have a due date on both rails, so absence here has to resolve to a
// number.
const DefaultPaymentTermsDays = 30

// paymentTermsDay is what one day of payment terms is: 24 hours, a
// duration, and never a calendar day in anybody's timezone (#1626).
const paymentTermsDay = 24 * time.Hour

// DueAfterTerms is the rule for when an Invoice falls due, stated once:
// netDays x 24 hours after the instant it was raised, to the second.
// netDays is the Practice's payment terms, or DefaultPaymentTermsDays at
// a Practice that has set none.
//
// It is a duration and not netDays calendar days, so the answer does not
// depend on a timezone -- not the Practice's, not the Postgres session's,
// not this process's. Across a daylight-saving change the two readings
// differ by an hour, and three records settle which one this is:
//
//   - ADR-0038 makes the due date an instant, and derives Overdue from an
//     instant comparison (due_at < now()). There is no calendar day in
//     it. ADR-0036's calendar day is about date columns and names its own
//     scope; due_at is a timestamptz.
//   - ADR-0038 took 30 days from what the Stripe rail already applied as
//     days_until_due, and Stripe counts that day as 86,400 seconds:
//     measured on the Sandbox on 2026-10-02, days_until_due=30 gave a
//     due_date exactly 2,592,000 seconds after created, across the
//     25-hour day of 2026-11-01.
//   - The app counts days late the same way, in whole 24-hour periods
//     from the due instant.
//
// The second is the resolution Stripe's own due_date carries: the value
// sent to Stripe as a Unix timestamp and the value stored on the row are
// then literally the same instant, rather than the same instant to within
// a microsecond. Truncate works on absolute time, so the cut does not
// depend on raisedAt's Location either.
func DueAfterTerms(raisedAt time.Time, netDays int) time.Time {
	return raisedAt.Truncate(time.Second).Add(time.Duration(netDays) * paymentTermsDay)
}

// MsgNetDaysOutOfRange is what a person reads when the terms she typed
// are not a usable number of days -- docs/api-design.md section 7 rule 4:
// a 4xx she can cause by filling in a form names the field at fault,
// starts with the field's own noun ("Days to pay", the label the
// settings screen gives it), says what to do, and avoids the four words
// apierr's TestDetailsWording gates on. The app spends the same sentence
// (invoice.ts's netDaysOutOfRangeMessage), so the refusal she meets
// before the request and the one she meets after it are one sentence.
const MsgNetDaysOutOfRange = "Days to pay must be a whole number of days from 1 to 365"

// maxPaymentTermsDays bounds what PutPaymentTermsHandler accepts, matching
// the CHECK constraint in 00102_invoice_due_at.sql. A year is far past
// anything a Practice bills a Client on; the bound exists so a typo
// ("300" for "30") cannot put an Invoice's due date beyond any horizon a
// Practice would notice.
const maxPaymentTermsDays = 365

// actionPaymentTermsChanged records a change to a Practice's payment
// terms -- Practice-scoped, plain-string, the same shape
// actionBillingModeChanged uses.
const actionPaymentTermsChanged = "payment_terms_changed"

// PaymentTermsResponse is what both handlers return. NetDays is always a
// usable number, whether or not the Practice has set one; IsDefault says
// which of the two it is, so a settings screen can show "30 days
// (default)" rather than making a Practice that never chose look
// indistinguishable from one that deliberately chose 30.
type PaymentTermsResponse struct {
	NetDays   int  `json:"netDays"`
	IsDefault bool `json:"isDefault"`
}

// PaymentTermsPutRequest is PutPaymentTermsHandler's body: PUT semantics,
// replacing the Practice's whole terms setting.
type PaymentTermsPutRequest struct {
	NetDays int `json:"netDays"`
}

// termsDiff is PutPaymentTermsHandler's activity diff. NetDaysBefore is
// nil when the Practice had never set terms of its own -- the reader is
// then told the Practice moved off the default, which is a different act
// from changing one chosen number to another.
type termsDiff struct {
	NetDaysBefore *int `json:"netDaysBefore"`
	NetDaysAfter  int  `json:"netDaysAfter"`
}

// fetchPaymentTerms reads practiceID's payment terms, reporting whether
// the Practice has set any of its own. The returned day count is usable
// either way: an unset Practice reads back DefaultPaymentTermsDays.
func fetchPaymentTerms(ctx context.Context, tx *sql.Tx, practiceID string) (netDays int, set bool, err error) {
	var stored int
	err = tx.QueryRowContext(ctx,
		`SELECT net_days FROM practice_payment_terms WHERE practice_id = $1`, practiceID,
	).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultPaymentTermsDays, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, false, fmt.Errorf("payments: read payment terms: %w", err)
	}
	return stored, true, nil
}

// invoiceDueAt is the due date an Invoice raised right now carries, from
// the Practice's own terms. The instant comes from Postgres rather than
// Go so that it agrees with created_at on the same row -- both are the
// database's clock, which is also the clock simclock shifts when a
// sandbox runs compressed time (docs/research/simulated-clock-compression.md).
//
// Postgres supplies the instant and nothing else. The terms are added in
// Go, by DueAfterTerms, because Postgres adds a days interval to a
// timestamptz as calendar days in the session's TimeZone: the same
// statement gives an instant one hour different on a connection whose
// zone has daylight saving time, and nothing in this repo sets that zone
// (#1626).
//
// One value, computed once: the Stripe rail sends this exact timestamp to
// Stripe as its due_date and stores the same one, rather than letting
// Stripe derive a second date of its own from a day count.
func invoiceDueAt(ctx context.Context, tx *sql.Tx, practiceID string) (time.Time, error) {
	netDays, _, err := fetchPaymentTerms(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return time.Time{}, err
	}
	var raisedAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&raisedAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return time.Time{}, fmt.Errorf("payments: read the instant an invoice is raised: %w", err)
	}
	return DueAfterTerms(raisedAt, netDays), nil
}

// GetPaymentTermsHandler lets any Staff member at the current Practice
// read its payment terms -- the same reasoning GetBillingModeHandler
// gives for the sibling billing-rail fact: a Doula meets the due date on
// every Invoice she looks at, so the terms behind it are not a number
// only an Owner may see. Must be mounted behind staffauth.Middleware.
func GetPaymentTermsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		netDays, set, err := fetchPaymentTerms(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, PaymentTermsResponse{NetDays: netDays, IsDefault: !set})
	})
}

// PutPaymentTermsHandler sets or changes how many days a Practice gives a
// Client to pay. Owner and Admin only, declared at the mount (#990), the
// same pair that may record a Payment or write an Invoice off: the terms
// decide when the Practice's own book calls money late.
//
// Idempotent by construction, following practicerate.PutRateHandler: a
// retry with the same body reads the same stored value, writes nothing,
// and records nothing new. Changing the terms never moves an Invoice
// already raised -- due_at is written onto the row at the moment it is
// raised, so an Invoice keeps the terms it was billed under, the same way
// it keeps the rail it was born on.
//
// Must be mounted behind staffauth.Middleware.
func PutPaymentTermsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var req PaymentTermsPutRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.NetDays < 1 || req.NetDays > maxPaymentTermsDays {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				fmt.Sprintf("netDays must be between 1 and %d", maxPaymentTermsDays),
				map[string]string{"netDays": MsgNetDaysOutOfRange})
			return
		}

		before, set, err := fetchPaymentTerms(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// An unset Practice writing 30 is still a real change: it moves
		// off "whatever DoulaCloud picks" onto a number it chose, and
		// the Activity row is the only record that it did.
		if !set || before != req.NetDays {
			if err := writePaymentTerms(r.Context(), tx, practiceID, req.NetDays, before, set); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, PaymentTermsResponse{NetDays: req.NetDays, IsDefault: false})
	})
}

// writePaymentTerms persists the terms and records who changed them and
// when -- one write path, so the row and its Activity entry can never
// disagree about what happened.
func writePaymentTerms(ctx context.Context, tx *sql.Tx, practiceID string, netDays, before int, hadOwnTerms bool) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_payment_terms (practice_id, net_days) VALUES ($1, $2)
		 ON CONFLICT (practice_id) DO UPDATE SET net_days = EXCLUDED.net_days`,
		practiceID, netDays,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: write payment terms: %w", err)
	}

	diff := termsDiff{NetDaysAfter: netDays}
	if hadOwnTerms {
		diff.NetDaysBefore = &before
	}
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
		return fmt.Errorf("payments: marshal payment terms diff: %w", err)
	}

	actorStaffID, _ := staffauth.StaffID(ctx)
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionPaymentTermsChanged,
		Diff:        diffJSON,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record payment terms change: %w", err)
	}
	return nil
}
