// Package staffnotice is the Platform Notification (ADR-0009) a Staff
// member gets when another person does something that concerns her at
// one of her Practices (#2267). One outbox, staff_notice_outbox, holds
// every kind; a kind adds a value to staff_notice_kind and a case to
// compose, and nothing else.
package staffnotice

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/outbox"
)

// Kind is what happened, one staff_notice_kind value.
type Kind string

// KindOfferReceived is an Offer made to someone already at the Practice.
// Its subject is the engagement_offers row.
const KindOfferReceived Kind = "offer_received"

// Notice is one person to tell about one record.
type Notice struct {
	PracticeID       string
	RecipientStaffID string
	Kind             Kind
	SubjectID        string
	TriggeredBy      string
}

// Queue writes a pending row for n in tx, the transaction that made the
// change it reports, so the notice survives exactly when the change does.
// A second Queue for the same person and record while one is pending
// writes nothing.
func Queue(ctx context.Context, tx *sql.Tx, n Notice) error {
	now := clock.Now(ctx)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_notice_outbox
		     (practice_id, recipient_staff_id, kind, subject_id, triggered_by_staff_id, next_attempt_at, created_at)
		 VALUES ($1, $2, $3::staff_notice_kind, $4, NULLIF($5, '')::uuid, $6, $6)
		 ON CONFLICT (kind, subject_id, recipient_staff_id) WHERE status = 'pending' DO NOTHING`,
		n.PracticeID, n.RecipientStaffID, string(n.Kind), n.SubjectID, n.TriggeredBy, now,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffnotice: queue: %w", err)
	}
	return nil
}

// Worker sends due staff_notice_outbox rows through outbox.MailWorker.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the Staff-notice worker around mailer.
func NewWorker(mailer outbox.Mailer) Worker {
	return Worker{
		Mailer:     mailer,
		Table:      "staff_notice_outbox",
		ClaimQuery: claimQuery,
		Scan:       scanRow,
		Compose:    compose(mailer),
	}
}

type pendingRow struct {
	id         string
	practiceID string
	kind       Kind
	subjectID  string
	address    string
	isMember   bool
	timezone   string
}

const claimQuery = `SELECT o.id, o.attempt_count, o.id, o.practice_id, o.kind::text, o.subject_id, s.email,
	        EXISTS (SELECT 1 FROM practice_memberships pm
	                 WHERE pm.practice_id = o.practice_id AND pm.staff_id = o.recipient_staff_id),
	        p.timezone
	   FROM staff_notice_outbox o
	   JOIN staff s ON s.id = o.recipient_staff_id
	   JOIN practices p ON p.id = o.practice_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var p pendingRow
	var kind string
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &p.id, &p.practiceID, &kind, &p.subjectID,
		&p.address, &p.isMember, &p.timezone); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, p, fmt.Errorf("staffnotice: scan outbox row: %w", err)
	}
	p.kind = Kind(kind)
	return meta, p, nil
}

// compose rechecks the row at send time: a person who has left the
// Practice, or a record already resolved, is marked sent with nothing
// mailed. The address it goes to is written on the row before the send,
// so the row says whom it reached; a retry overwrites it.
func compose(mailer outbox.Mailer) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(ctx context.Context, tx *sql.Tx, p pendingRow, now time.Time) (string, string, string, error) {
		if !p.isMember {
			return "", "", "", outbox.ErrAlreadyDone
		}
		var subject, text string
		switch p.kind {
		case KindOfferReceived:
			expiresAt, open, err := openOffer(ctx, tx, p.subjectID, now)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return "", "", "", err
			}
			if !open {
				return "", "", "", outbox.ErrAlreadyDone
			}
			link := mailer.AppBaseURL + "/practices/" + p.practiceID + "/offers"
			subject, text = offerReceivedSubject, offerReceivedText(link, inZone(expiresAt, p.timezone))
		default:
			// coverage:ignore reason: staff_notice_kind admits no value without a case here
			return "", "", "", &outbox.DeadLetterError{Reason: "unknown staff notice kind " + string(p.kind)}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE staff_notice_outbox SET sent_to_address = $2 WHERE id = $1`, p.id, p.address,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return "", "", "", fmt.Errorf("staffnotice: record address: %w", err)
		}
		return p.address, subject, text, nil
	}
}

// openOffer reports whether the Offer is still open at now, and when it
// expires. Read through 00041's engagement_offers notification-worker
// policy.
func openOffer(ctx context.Context, tx *sql.Tx, offerID string, now time.Time) (time.Time, bool, error) {
	var state string
	var expiresAt time.Time
	err := tx.QueryRowContext(ctx,
		`SELECT state::text, expires_at FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&state, &expiresAt)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return time.Time{}, false, fmt.Errorf("staffnotice: read offer: %w", err)
	}
	return expiresAt, state == "offered" && expiresAt.After(now), nil
}

// inZone is t in the Practice's own zone (ADR-0036), or UTC for a zone
// this binary cannot load.
func inZone(t time.Time, zone string) time.Time {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		// coverage:ignore reason: practices.timezone is validated on write, so an unloadable zone is not seeded by any test
		return t.UTC()
	}
	return t.In(loc)
}
