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

const noDoulaPageSize = 30

// The two ways work comes to have no Doula, and the Overview's line for
// each.
const (
	ReasonDeclined = "declined"
	ReasonTakenOff = "taken_off"
)

// NoDoula is one Engagement that waits for a Doula (#2508, #2596): who
// said no, or who was taken off, to which Client's work, and when. ID is
// the declined Offer's, or the taking-off's activity entry's.
type NoDoula struct {
	ID           string    `json:"id"`
	EngagementID string    `json:"engagementId"`
	ClientName   string    `json:"clientName"`
	Kind         string    `json:"kind"`
	Reason       string    `json:"reason"`
	Name         string    `json:"name"`
	At           time.Time `json:"at"`
}

// NoDoulaResponse is the cursor-pagination envelope of
// docs/api-design.md section 4.
type NoDoulaResponse struct {
	Items      []NoDoula `json:"items"`
	NextCursor *string   `json:"nextCursor,omitempty"`
	HasMore    bool      `json:"hasMore"`
}

// NoDoulaHandler serves the Overview's "No Doula yet" (#2508, #2596): each
// Engagement that is not completed, that nobody holds an open granted
// Attachment on, and that has no Offer still open, with the last thing
// that left it so. That is a declined Offer, or the Doula an Owner or an
// Admin took off it; only the newest taking-off of an Engagement counts.
// Neither mails anybody (GLOSSARY.md's Offer entry); this is where an
// Owner or an Admin reads it, and the mount holds it to them.
func NoDoulaHandler() http.Handler {
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
				apierr.WriteError(w, "That page of work without a Doula cannot be found. Load the list again.", http.StatusBadRequest)
				return
			}
			afterAt, afterID = &c.At, &c.ID
		}

		items, err := listNoDoula(r, tx, practiceID, afterAt, afterID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp := NoDoulaResponse{Items: items}
		if len(items) > noDoulaPageSize {
			resp.Items, resp.HasMore = items[:noDoulaPageSize], true
			last := resp.Items[noDoulaPageSize-1]
			next := pagecursor.Encode(last.At, last.ID)
			resp.NextCursor = &next
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// noDoulaSelect is the declined Offers and the takings-off, each already
// narrowed to an Engagement nobody holds. $1 is the Practice and $2 is now.
const noDoulaSelect = `SELECT id, engagement_id, kind, given_name, preferred_name, family_name, reason, name, at FROM (
	SELECT o.id, e.id AS engagement_id, e.kind::text AS kind, c.given_name, c.preferred_name, c.family_name,
	       'declined' AS reason, COALESCE(s.name, pi.address, '') AS name, o.decided_at AS at
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
	UNION ALL
	SELECT a.id, e.id, e.kind::text, c.given_name, c.preferred_name, c.family_name,
	       'taken_off', COALESCE(s.name, ''), a.created_at
	  FROM activity a
	  JOIN engagements e ON e.id = a.subject_id
	  JOIN clients c ON c.id = e.client_id
	  LEFT JOIN staff s ON s.id = (a.diff ->> 'attachedStaffId')::uuid
	 WHERE a.practice_id = $1 AND a.subject_kind = 'engagement' AND a.action = 'doula_taken_off'
	   AND e.status <> 'completed'
	   AND NOT EXISTS (SELECT 1 FROM activity later
	                    WHERE later.subject_kind = 'engagement' AND later.subject_id = a.subject_id
	                      AND later.action = 'doula_taken_off' AND (later.created_at, later.id) > (a.created_at, a.id))
	   AND NOT EXISTS (SELECT 1 FROM engagement_attachments ea
	                    WHERE ea.engagement_id = e.id AND ea.origin = 'granted' AND ea.ended_at IS NULL)
	   AND NOT EXISTS (SELECT 1 FROM engagement_offers open
	                    WHERE open.engagement_id = e.id AND open.state = 'offered' AND open.expires_at > $2)
) x
WHERE ($3::timestamptz IS NULL OR (at, id) < ($3::timestamptz, $4::uuid))
ORDER BY at DESC, id DESC
LIMIT $5`

func listNoDoula(r *http.Request, tx *sql.Tx, practiceID string, afterAt *time.Time, afterID *string) ([]NoDoula, error) {
	rows, err := tx.QueryContext(r.Context(), noDoulaSelect,
		practiceID, clock.Now(r.Context()), afterAt, afterID, noDoulaPageSize+1,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: list no doula: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []NoDoula{}
	for rows.Next() {
		var d NoDoula
		var given string
		var preferred, family sql.NullString
		if err := rows.Scan(&d.ID, &d.EngagementID, &d.Kind, &given, &preferred, &family, &d.Reason, &d.Name, &d.At); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("offer: scan no doula: %w", err)
		}
		d.ClientName = personname.Full(given, preferred.String, family.String)
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: iterate no doula: %w", err)
	}
	return items, nil
}
