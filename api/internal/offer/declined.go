package offer

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/staffauth"
)

const declinedPageSize = 30

// Declined is one declined Offer whose Engagement still waits for a
// Doula (#2508): who said no, to which Client's work, and when.
type Declined struct {
	OfferID      string    `json:"offerId"`
	EngagementID string    `json:"engagementId"`
	ClientName   string    `json:"clientName"`
	Kind         string    `json:"kind"`
	DeclinedBy   string    `json:"declinedBy"`
	DeclinedAt   time.Time `json:"declinedAt"`
}

// DeclinedResponse is the cursor-pagination envelope of
// docs/api-design.md section 4.
type DeclinedResponse struct {
	Items      []Declined `json:"items"`
	NextCursor *string    `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}

// DeclinedHandler serves the Overview's declined Offers (#2508): each
// declined Offer on an Engagement that is not completed, that nobody holds
// an open granted Attachment on, and that has no Offer still open. A
// decline mails nobody (GLOSSARY.md's Offer entry); this is where an Owner
// or an Admin reads it, and the mount holds it to them.
func DeclinedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		var afterAt *time.Time
		var afterID *string
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "That page of declined offers cannot be found. Load the list again.", http.StatusBadRequest)
				return
			}
			afterAt, afterID = &c.At, &c.ID
		}

		items, err := listDeclined(r, tx, practiceID, afterAt, afterID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp := DeclinedResponse{Items: items}
		if len(items) > declinedPageSize {
			resp.Items, resp.HasMore = items[:declinedPageSize], true
			last := resp.Items[declinedPageSize-1]
			next := pagecursor.Encode(last.DeclinedAt, last.OfferID)
			resp.NextCursor = &next
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func listDeclined(r *http.Request, tx *sql.Tx, practiceID string, afterAt *time.Time, afterID *string) ([]Declined, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT o.id, e.id, e.kind::text, c.given_name, c.preferred_name, c.family_name,
		        COALESCE(s.name, pi.address, ''), o.decided_at
		   FROM engagement_offers o
		   JOIN engagements e ON e.id = o.engagement_id
		   JOIN clients c ON c.id = e.client_id
		   LEFT JOIN staff s ON s.id = o.staff_id
		   LEFT JOIN practice_invitations pi ON pi.id = o.invitation_id
		  WHERE e.practice_id = $1 AND o.state = 'declined' AND e.status <> 'completed'
		    AND NOT EXISTS (SELECT 1 FROM engagement_attachments ea
		                     WHERE ea.engagement_id = e.id AND ea.origin = 'granted' AND ea.ended_at IS NULL)
		    AND NOT EXISTS (SELECT 1 FROM engagement_offers open
		                     WHERE open.engagement_id = e.id AND open.state = 'offered' AND open.expires_at > $2)
		    AND ($3::timestamptz IS NULL OR (o.decided_at, o.id) < ($3::timestamptz, $4::uuid))
		  ORDER BY o.decided_at DESC, o.id DESC
		  LIMIT $5`,
		practiceID, clock.Now(r.Context()), afterAt, afterID, declinedPageSize+1,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: list declined: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []Declined{}
	for rows.Next() {
		var d Declined
		var given string
		var preferred, family sql.NullString
		if err := rows.Scan(&d.OfferID, &d.EngagementID, &d.Kind, &given, &preferred, &family, &d.DeclinedBy, &d.DeclinedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("offer: scan declined: %w", err)
		}
		d.ClientName = personname.Full(given, preferred.String, family.String)
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: iterate declined: %w", err)
	}
	return items, nil
}
