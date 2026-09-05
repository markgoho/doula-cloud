package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
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

// The messages the roll-up returns, named because the screen renders them
// and the tests assert them.
const (
	MsgInvalidCursor = "invalid cursor"
	MsgInternalError = "internal error"
)

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
// Owner or Admin only, which #426 asked for and which is worth stating
// against ADR-0008 rather than assumed: the read table's "Contract --
// scope" row does admit an employee Doula, and nothing returned here is
// money. But that row is about a Contract she reaches through an
// Engagement, and the contractor half of the same table narrows even
// that to the Engagements she is attached to. A Practice-wide roll-up
// has no Engagement to reach through -- it names every Client with an
// agreement outstanding anywhere at the Practice, attached or not -- so
// it follows the Practice-wide readers the table does have: the credit
// balance and the Practice-wide Invoice list, both Owner and Admin.
// GetContractHandler is still where a Doula reads the scope of a
// Contract she is on.
//
// Ordered oldest first, which is what makes it a work list rather than a
// feed: the Contract that has been waiting longest is the one that has
// cost the most, and it belongs at the top.
func AwaitingSignatureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
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

		list, err := listAwaiting(r.Context(), tx, practiceID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
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

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			// coverage:ignore reason: response encoding failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// listAwaiting reads one page of outstanding Contracts, oldest first. The
// cursor comparison is `>` because this list ascends; pagecursor carries
// a position, not a direction.
//
// 'draft' and 'sent' are the whole predicate. A signed Contract is done,
// and a voided one is a superseded record rather than outstanding work --
// it stays unsigned forever and no amount of chasing changes that.
// Excluding voided also means each Engagement appears at most once, for
// free: 00020's partial unique index already allows only one non-voided
// Contract per Engagement, so nothing here has to de-duplicate.
//
// contracts has no practice_id column, so the Practice filter is the join
// to engagements -- filtered explicitly on top of the RLS scoping
// staffauth.Middleware already set on tx, the same belt-and-braces
// engagementrequest's inbox uses. The join to clients is for her name
// only.
func listAwaiting(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]AwaitingItem, error) {
	query := `SELECT c.id, e.id, cl.id, cl.given_name, cl.preferred_name, c.status::text, c.created_at
	            FROM contracts c
	            JOIN engagements e ON e.id = c.engagement_id
	            JOIN clients cl ON cl.id = e.client_id
	           WHERE e.practice_id = $1 AND c.status IN ('draft', 'sent')`
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
