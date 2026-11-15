// Package contractnotice is the Practice Notification (ADR-0009) a Client
// gets about her Contract (#2951): one when it is sent, and one each time
// Staff remind her. One outbox, contract_notice_outbox, holds both kinds;
// the mail is content-free (no name, no Engagement detail, no Practice
// name) and carries one link.
package contractnotice

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/portalinvite"
)

// Kind is what the mail is, one contract_notice_kind value.
type Kind string

const (
	// KindSent is the mail at send.
	KindSent Kind = "sent"
	// KindReminder is a Staff member's reminder.
	KindReminder Kind = "reminder"
)

const table = "contract_notice_outbox"

// Queue writes a pending row for contractID in tx, the transaction that
// made the change it reports, so the mail survives exactly when the change
// does. triggeredBy is the Staff member who sent or reminded. A second
// Queue of the same kind while one is pending writes nothing.
func Queue(ctx context.Context, tx *sql.Tx, practiceID, contractID string, kind Kind, triggeredBy string) error {
	now := clock.Now(ctx)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contract_notice_outbox (practice_id, contract_id, kind, triggered_by_staff_id, next_attempt_at, created_at)
		 VALUES ($1, $2, $3::contract_notice_kind, $4, $5, $5)
		 ON CONFLICT (kind, contract_id) WHERE status = 'pending' DO NOTHING`,
		practiceID, contractID, string(kind), triggeredBy, now,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contractnotice: queue: %w", err)
	}
	return nil
}

// Worker sends due contract_notice_outbox rows through outbox.MailWorker.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the Contract-notice worker around mailer.
func NewWorker(mailer outbox.Mailer) Worker {
	return Worker{
		Mailer:     mailer,
		Table:      table,
		ClaimQuery: claimQuery,
		Scan:       scanRow,
		Compose:    compose(mailer),
	}
}

type pendingRow struct {
	id           string
	kind         Kind
	status       string
	engagementID string
	email        sql.NullString
	state        sql.NullString
	expiresAt    sql.NullTime
	delivery     sql.NullString
	inviteToken  sql.NullString
}

// The claim reads the Contract, her Engagement and her access through the
// worker door (00182 for contracts and engagements, 00032 for clients and
// client_portal_users), joined by the same projection the Engagement
// page's readout and the send itself use.
const claimQuery = `SELECT o.id, o.attempt_count, o.id, o.kind::text, ct.status::text, e.id, c.email,
	        ` + portalinvite.HerAccessColumns + `,
	        (SELECT t.invite_token::text FROM client_portal_users t WHERE t.id = pu.id)
	   FROM contract_notice_outbox o
	   JOIN contracts ct ON ct.id = o.contract_id
	   JOIN engagements e ON e.id = ct.engagement_id
	   JOIN clients c ON c.id = e.client_id
	   ` + portalinvite.HerAccessJoin + `
	  ` + outbox.ClaimNextAliasO

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var p pendingRow
	var kind string
	meta, err := outbox.ScanMeta(rows, &p.id, &kind, &p.status, &p.engagementID, &p.email,
		&p.state, &p.expiresAt, &p.delivery, &p.inviteToken)
	if err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, p, fmt.Errorf("contractnotice: scan outbox row: %w", err)
	}
	p.kind = Kind(kind)
	return meta, p, nil
}

// compose rechecks the row at send time. A Contract no longer Sent (signed,
// voided) has nothing left to say and is skipped. A Client with no address
// on file, or no way into the portal, is dead-lettered: nothing about
// waiting fixes either. The address it goes to is written on the row before
// the send, so the row says whom it reached; a retry overwrites it.
//
// She is taken where she can go. With a login she signs in already, so the
// link is her Contract, and the portal asks her to sign in first if she
// has not. Without one she holds an invitation (send refuses a Client with
// none), so the link is that invitation, read fresh here as portalinvite's
// own mail does, and accepting it lands her in the portal.
func compose(mailer outbox.Mailer) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(ctx context.Context, tx *sql.Tx, p pendingRow, now time.Time) (string, string, string, error) {
		if p.status != "sent" {
			return "", "", "", outbox.ErrAlreadyDone
		}
		if !p.email.Valid || p.email.String == "" {
			return "", "", "", &outbox.DeadLetterError{Reason: "client has no email on file"}
		}
		access := portalinvite.Derive(p.state, p.expiresAt, p.delivery, now)
		if !access.CanReachPortal() {
			return "", "", "", &outbox.DeadLetterError{Reason: "client has no way into the portal"}
		}
		link := mailer.AppBaseURL + "/portal/engagements/" + p.engagementID + "/contract"
		if access.State != portalinvite.StateAccepted {
			if p.inviteToken.String == "" {
				// coverage:ignore reason: a live invitation always carries its token (00032)
				return "", "", "", &outbox.DeadLetterError{Reason: "invitation has no token"}
			}
			link = mailer.AppBaseURL + "/portal/accept-invite?token=" + p.inviteToken.String
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE contract_notice_outbox SET sent_to_address = $2 WHERE id = $1`, p.id, p.email.String,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return "", "", "", fmt.Errorf("contractnotice: record address: %w", err)
		}
		subject, text := copyFor(p.kind, link)
		return p.email.String, subject, text, nil
	}
}
