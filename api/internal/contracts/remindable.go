package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/portalinvite"
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

// The reasons a reminder is withheld, as the list and the Engagement carry
// them in remindWithheld. The screen words each one; the endpoint refuses
// with the sentence in remindRule.Refusal.
const (
	withheldEmailedToday = "emailed_today"
	withheldOnItsWay     = "on_its_way"
	withheldNotInvited   = "not_invited"
	withheldInviteLost   = "invite_lost"
	withheldNoAccess     = "no_access"
	withheldErased       = "erased"
	withheldMerged       = "merged"
)

// msgRemindOnItsWay is the refusal while an earlier reminder for this
// Contract is still queued (retrying), whichever day it was asked for.
const msgRemindOnItsWay = "A reminder for this Contract is already on its way."

// remindRule is what the BFF will do with a reminder for one Contract: send
// it (Can), or withhold it for Why. On is the Practice-zone day it can go
// when the only thing in the way is today's mail. Refusal is the sentence
// the endpoint refuses with.
type remindRule struct {
	Can     bool
	On      string
	Why     string
	Refusal string
}

// remindTarget is one Sent Contract and the Client it would mail.
type remindTarget struct{ ContractID, ClientID string }

// remindRules is the one rule for a reminder, read by the endpoint, the
// Contracts list and the Engagement alike, so Remind is offered exactly
// where pressing it will mail. It is keyed by Contract id. A reminder is
// withheld, in this order, for a Client who is erased or merged away, one
// the portal cannot reach, one already emailed today by a send or a
// reminder, and a Contract whose earlier reminder is still queued.
func remindRules(ctx context.Context, tx *sql.Tx, targets []remindTarget, zone *time.Location) (map[string]remindRule, error) {
	out := make(map[string]remindRule, len(targets))
	if len(targets) == 0 {
		return out, nil
	}
	clientIDs := make([]string, len(targets))
	contractIDs := make([]string, len(targets))
	for i, t := range targets {
		clientIDs[i], contractIDs[i] = t.ClientID, t.ContractID
	}
	now := clock.Now(ctx)
	reached, err := clientsReach(ctx, tx, clientIDs, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	emailed, err := clientsEmailedSince(ctx, tx, clientIDs, startOfDay(now, zone))
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	queued, err := contractsWithQueuedReminder(ctx, tx, contractIDs)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	tomorrow := startOfDay(now, zone).AddDate(0, 0, 1).Format(dayLayout)
	for _, t := range targets {
		switch r := reached[t.ClientID]; {
		case r.withheld != "":
			out[t.ContractID] = remindRule{Why: r.withheld, Refusal: r.refusal}
		case emailed[t.ClientID]:
			out[t.ContractID] = remindRule{Why: withheldEmailedToday, On: tomorrow, Refusal: msgRemindedToday}
		case queued[t.ContractID]:
			out[t.ContractID] = remindRule{Why: withheldOnItsWay, Refusal: msgRemindOnItsWay}
		default:
			out[t.ContractID] = remindRule{Can: true}
		}
	}
	return out, nil
}

const clientsReachQuery = `SELECT c.id::text, c.standing::text, ` + portalinvite.HerAccessColumns + `
	   FROM clients c` + portalinvite.HerAccessJoin + `
	  WHERE c.id::text = ANY($1)`

// reach is whether a reminder can reach a Client at all and, when not, why.
type reach struct{ withheld, refusal string }

// clientsReach reads, for each of clientIDs, whether a reminder can reach
// her: she must be a live record, and the portal must be open to her.
func clientsReach(ctx context.Context, tx *sql.Tx, clientIDs []string, now time.Time) (map[string]reach, error) {
	rows, err := tx.QueryContext(ctx, clientsReachQuery, clientIDs)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: read clients' reach: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]reach{}
	for rows.Next() {
		var (
			id, standing     string
			stored, delivery sql.NullString
			expiresAt        sql.NullTime
		)
		if err := rows.Scan(&id, &standing, &stored, &expiresAt, &delivery); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan client's reach: %w", err)
		}
		out[id] = reachOf(clientstanding.Standing(standing), portalinvite.Derive(stored, expiresAt, delivery, now))
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate clients' reach: %w", err)
	}
	return out, nil
}

func reachOf(s clientstanding.Standing, access *portalinvite.Access) reach {
	if ok, refusal := clientstanding.ActRemindContract.Check(s); !ok {
		if s.IsTombstone() {
			return reach{withheldMerged, refusal}
		}
		return reach{withheldErased, refusal}
	}
	if access.CanReachPortal() {
		return reach{}
	}
	refusal := remindRefusal(access)
	switch {
	case access == nil:
		return reach{withheldNotInvited, refusal}
	case access.State == portalinvite.StateRefused || access.State == portalinvite.StateUnlinked:
		return reach{withheldNoAccess, refusal}
	}
	return reach{withheldInviteLost, refusal}
}

// contractsWithQueuedReminder reports which of contractIDs has a reminder
// still pending, from any day.
func contractsWithQueuedReminder(ctx context.Context, tx *sql.Tx, contractIDs []string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT contract_id::text FROM contract_notice_outbox
		  WHERE kind = 'reminder' AND status = 'pending' AND contract_id::text = ANY($1)`, contractIDs)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: read queued reminders: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan queued reminder: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate queued reminders: %w", err)
	}
	return out, nil
}

// remindStateOf is the rule for one Engagement's Sent Contract, read for
// the Engagement page.
func remindStateOf(ctx context.Context, tx *sql.Tx, practiceID, engagementID string) (remindRule, error) {
	zone, err := practicetimezone.Load(ctx, tx, practiceID)
	if err != nil {
		return remindRule{}, fmt.Errorf("contracts: load practice zone: %w", err)
	}
	t, err := sentTarget(ctx, tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return remindRule{}, err
	}
	rules, err := remindRules(ctx, tx, []remindTarget{t}, zone)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return remindRule{}, err
	}
	return rules[t.ContractID], nil
}

// sentTarget is the Engagement's current Contract and its Client.
func sentTarget(ctx context.Context, tx *sql.Tx, engagementID string) (remindTarget, error) {
	var t remindTarget
	if err := tx.QueryRowContext(ctx,
		`SELECT ct.id::text, e.client_id::text FROM contracts ct JOIN engagements e ON e.id = ct.engagement_id
		  WHERE ct.engagement_id = $1 ORDER BY ct.created_at DESC, ct.id DESC LIMIT 1`, engagementID,
	).Scan(&t.ContractID, &t.ClientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- every caller has just read this Engagement's Sent Contract
		return t, fmt.Errorf("contracts: resolve engagement's sent contract: %w", err)
	}
	return t, nil
}
