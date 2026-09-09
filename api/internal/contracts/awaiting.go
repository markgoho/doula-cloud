package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// awaitingPageSize is the fixed number of outstanding Contracts returned
// per page, matching engagementrequest's inbox: a fixed size keeps the
// query parameter surface small, and this is a work list somebody clears,
// not a report somebody scrolls.
const awaitingPageSize = 30

// MsgInvalidCursor is the roll-up's own refusal message; the screen
// renders it and the tests assert it.
const MsgInvalidCursor = "invalid cursor"

// AwaitingItem is one row of the "Contracts awaiting signature" list: the
// Engagement the Contract hangs off, the Client whose signature is
// outstanding, and how far along it is. The Client's name arrives already
// resolved -- the list names her, it does not print her record -- and
// EngagementID is what the row links to, every Contract screen being
// addressed by Engagement id alone.
//
// Status is "draft" or "sent", which is the difference between work the
// Practice still owes and work the Client still owes; a chaser needs to
// know which of the two she is looking at before she picks up the phone.
type AwaitingItem struct {
	EngagementID string    `json:"engagementId"`
	ContractID   string    `json:"contractId"`
	ClientID     string    `json:"clientId"`
	ClientName   string    `json:"clientName"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// AwaitingResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type AwaitingResponse struct {
	Items      []AwaitingItem `json:"items"`
	NextCursor *string        `json:"nextCursor,omitempty"`
	HasMore    bool           `json:"hasMore"`
}

// AwaitingSignatureHandler lists every Contract at the Practice that is
// not yet signed -- the roll-up #426 was opened for, and the thing whose
// absence made chasing signatures mean opening every Engagement in turn
// (docs/journeys/non-doula-admin.md, DW-G5). Must be mounted behind
// staffauth.Middleware.
//
// AwaitingItem carries no amount, so this is not a money read and does
// not follow the Practice-wide money rows (credit balance, Invoice
// history). It follows ADR-0008's "Engagements, Visits, Messages" row
// instead, the same row message.AwaitingReplyHandler's own Practice-wide
// roll-up follows: an employee Doula sees every outstanding Contract at
// the Practice, since she reaches every Engagement; a contractor Doula
// sees only the ones on an Engagement she holds an open, granted
// attachment on (#973). Under #282's write table a Doula is the one who
// sends a Contract and then waits on the signature, so she needs this
// list of what she is waiting on.
//
// Ordered oldest first, which is what makes it a work list rather than a
// feed: the Contract that has been waiting longest is the one that has
// cost the most, and it belongs at the top.
func AwaitingSignatureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, MsgInvalidCursor, http.StatusBadRequest)
				return
			}
			after = &c
		}

		var list []AwaitingItem
		var err error
		if reader.IsContractor() {
			list, err = listAttachedAwaiting(r.Context(), tx, practiceID, staffID, after)
		} else {
			list, err = listAwaiting(r.Context(), tx, practiceID, after)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > awaitingPageSize
		if hasMore {
			list = list[:awaitingPageSize]
		}
		resp := AwaitingResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.CreatedAt, last.ContractID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// awaitingSelect is shared by both queries below: the columns and joins
// that make one AwaitingItem row. 'draft' and 'sent' are the whole status
// predicate -- a signed Contract is done, and a voided one is a
// superseded record rather than outstanding work; it stays unsigned
// forever and no amount of chasing changes that. Excluding voided also
// means each Engagement appears at most once, for free: 00020's partial
// unique index already allows only one non-voided Contract per
// Engagement, so nothing here has to de-duplicate. The join to clients
// is for her name only.
const awaitingSelect = `SELECT c.id, e.id, cl.id, cl.given_name, cl.preferred_name, c.status::text, c.created_at
	            FROM contracts c
	            JOIN engagements e ON e.id = c.engagement_id
	            JOIN clients cl ON cl.id = e.client_id
	           WHERE e.practice_id = $1 AND c.status IN ('draft', 'sent')`

// listAwaiting reads one page of outstanding Contracts across the whole
// Practice, oldest first -- the ambient-reach query for an Owner, Admin,
// or employee Doula (ADR-0008). The cursor comparison is `>` because
// this list ascends; pagecursor carries a position, not a direction.
//
// contracts has no practice_id column, so the Practice filter is the join
// to engagements -- filtered explicitly on top of the RLS scoping
// staffauth.Middleware already set on tx, the same belt-and-braces
// engagementrequest's inbox uses.
func listAwaiting(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]AwaitingItem, error) {
	query := awaitingSelect
	args := []any{practiceID}
	if after != nil {
		query += ` AND (c.created_at, c.id) > ($2, $3) ORDER BY c.created_at, c.id LIMIT $4`
		args = append(args, after.At, after.ID, awaitingPageSize+1)
	} else {
		query += ` ORDER BY c.created_at, c.id LIMIT $2`
		args = append(args, awaitingPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list awaiting signature: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAwaitingItems(rows)
}

// listAttachedAwaiting is listAwaiting narrowed to Engagements staffID
// holds an open (ended_at IS NULL), granted-origin engagement_attachments
// row on -- the literal predicate staffauth.Reader.CanAccessEngagement
// runs, applied here as a SQL filter the same way
// message.listAttachedAwaitingReply already does for its own Practice-wide,
// single-subject-kind roll-up (#973).
func listAttachedAwaiting(ctx context.Context, tx *sql.Tx, practiceID, staffID string, after *pagecursor.Cursor) ([]AwaitingItem, error) {
	query := awaitingSelect + `
	             AND EXISTS (
	                 SELECT 1 FROM engagement_attachments ea
	                 WHERE ea.engagement_id = e.id AND ea.staff_id = $2
	                   AND ea.origin = 'granted' AND ea.ended_at IS NULL
	             )`
	args := []any{practiceID, staffID}
	if after != nil {
		query += ` AND (c.created_at, c.id) > ($3, $4) ORDER BY c.created_at, c.id LIMIT $5`
		args = append(args, after.At, after.ID, awaitingPageSize+1)
	} else {
		query += ` ORDER BY c.created_at, c.id LIMIT $3`
		args = append(args, awaitingPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list attached awaiting signature: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAwaitingItems(rows)
}

func scanAwaitingItems(rows *sql.Rows) ([]AwaitingItem, error) {
	list := []AwaitingItem{}
	for rows.Next() {
		var item AwaitingItem
		var givenName string
		var preferredName sql.NullString
		if err := rows.Scan(&item.ContractID, &item.EngagementID, &item.ClientID,
			&givenName, &preferredName, &item.Status, &item.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan awaiting signature: %w", err)
		}
		item.ClientName = client.PreferredName(givenName, preferredName.String)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate awaiting signature: %w", err)
	}
	return list, nil
}
