package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/practicetimezone"
)

// A reminder's delivery, as the Engagement says it.
const (
	deliveryQueued       = "queued"
	deliverySent         = "sent"
	deliveryNotDelivered = "not_delivered"
)

// ContractReminder is the latest reminder a Staff member asked for, read
// from the contract_notice_outbox row that carries it: who asked, when,
// and what became of the mail. The Activity entry records the act; the row
// is what knows whether it arrived.
type ContractReminder struct {
	At       time.Time `json:"at"`
	By       string    `json:"by,omitempty"`
	Delivery string    `json:"delivery"`
}

// deliveryOf maps an outbox status to what a reader is told. A skipped
// row is one the Contract moved past before it went out; she was not
// emailed, so it reads as not delivered.
func deliveryOf(status string) string {
	switch status {
	case "pending":
		return deliveryQueued
	case "sent":
		return deliverySent
	default:
		return deliveryNotDelivered
	}
}

// latestReminder is the newest reminder asked for on contractID, whatever
// became of its mail, or nil when none has been.
func latestReminder(ctx context.Context, tx *sql.Tx, contractID string) (*ContractReminder, error) {
	r, found, err := readLatestReminder(ctx, tx, contractID)
	if err != nil || !found {
		return nil, err
	}
	return &r, nil
}

func readLatestReminder(ctx context.Context, tx *sql.Tx, contractID string) (r ContractReminder, found bool, err error) {
	var (
		status string
		by     sql.NullString
	)
	err = tx.QueryRowContext(ctx,
		`SELECT o.created_at, o.status::text, (SELECT s.name FROM staff s WHERE s.id = o.triggered_by_staff_id)
		   FROM contract_notice_outbox o
		  WHERE o.contract_id = $1 AND o.kind = 'reminder'
		  ORDER BY o.created_at DESC, o.id DESC LIMIT 1`, contractID,
	).Scan(&r.At, &status, &by)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return r, false, fmt.Errorf("contracts: read contract reminder: %w", err)
	}
	r.By, r.Delivery = by.String, deliveryOf(status)
	return r, true, nil
}

// clientsEmailedSince reports which of clientIDs have a Contract mail
// queued or sent at or after since, whichever of her Contracts it was
// about and whether it was the mail at send or a reminder. A mail that
// was dead-lettered or skipped never reached her and does not count, so
// it frees the day. This is the one place the once-a-day rule is read:
// the reminder endpoint, the Contracts list and the Engagement all ask it.
func clientsEmailedSince(ctx context.Context, tx *sql.Tx, clientIDs []string, since time.Time) (map[string]bool, error) {
	emailed := map[string]bool{}
	if len(clientIDs) == 0 {
		return emailed, nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT DISTINCT e.client_id::text
		   FROM contract_notice_outbox o
		   JOIN contracts ct ON ct.id = o.contract_id
		   JOIN engagements e ON e.id = ct.engagement_id
		  WHERE e.client_id::text = ANY($1) AND o.status IN ('pending', 'sent') AND o.created_at >= $2`,
		clientIDs, since)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: read mail to clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan mail to client: %w", err)
		}
		emailed[id] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate mail to clients: %w", err)
	}
	return emailed, nil
}

// remindability turns "was she emailed today" into what a Staff reader is
// shown: whether a reminder goes out now and, when it does not, the
// Practice-zone day it can.
func remindability(emailedToday bool, now time.Time, zone *time.Location) (can bool, on string) {
	if !emailedToday {
		return true, ""
	}
	return false, startOfDay(now, zone).AddDate(0, 0, 1).Format(dayLayout)
}

// remindStateOf is the rule for one Engagement's Sent Contract, read for
// the Engagement page.
func remindStateOf(ctx context.Context, tx *sql.Tx, practiceID, engagementID string) (can bool, on string, err error) {
	zone, err := practicetimezone.Load(ctx, tx, practiceID)
	if err != nil {
		return false, "", fmt.Errorf("contracts: load practice zone: %w", err)
	}
	var clientID string
	if err := tx.QueryRowContext(ctx, `SELECT client_id::text FROM engagements WHERE id = $1`, engagementID).Scan(&clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- every caller has just read this Engagement's Contract
		return false, "", fmt.Errorf("contracts: resolve engagement client: %w", err)
	}
	now := clock.Now(ctx)
	emailed, err := clientsEmailedSince(ctx, tx, []string{clientID}, startOfDay(now, zone))
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, "", err
	}
	can, on = remindability(emailed[clientID], now, zone)
	return can, on, nil
}
