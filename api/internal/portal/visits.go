package portal

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/pagecursor"
)

// visitPageSize matches activityPageSize's own reasoning -- there is
// nothing visit-specific about how many rows one page carries.
const visitPageSize = 30

// Visit is one row of the Client's own "Your visits" read (#478): when
// it is, and who is coming. That is the whole DTO, and every field the
// Staff-side visit.Visit carries and this one does not is a deliberate
// omission rather than an oversight:
//
//   - no staffId -- a Client is told a person's name, never the
//     Practice's own identifier for her;
//   - no notes -- staff-only, ADR-0006 and #251;
//   - no type -- CONTEXT.md's Visit entry keeps `prenatal`/`birth`/
//     `postpartum` staff-only and settles no Client word for it, and
//     `postpartum` "still describes a birth with a baby at the end of
//     it" (docs/journeys/loss-client.md), so labeling a bereavement
//     Visit in Nadia's own portal is the CB-G5 mistake this surface
//     exists to avoid;
//   - nothing from the Care Plan.
//
// ScheduledAt is not a pointer, unlike visit.Visit's: a Visit nobody has
// scheduled never reaches this response at all (see listPortalVisits), so
// every row here has one.
type Visit struct {
	VisitID     string    `json:"visitId"`
	ScheduledAt time.Time `json:"scheduledAt"`
	// DoulaName is the name of the Doula who is coming -- CONTEXT.md's
	// own settled word for this surface ("a Client sees the visits on her
	// own Engagement, past and scheduled, with who is coming").
	// Deliberately NOT the Activity ledger's redactStaffActorNames
	// redaction, even though both land on activity.StaffActorDisplayName:
	// that rule answers "never who inside the Practice did what" about a
	// Practice's own roster acts, and who is coming to her home is a fact
	// about her care, not about the roster. Here the same word is reached
	// for a different reason -- an unreadable row, not a redaction
	// policy -- which is why listPortalVisits still names the Doula by
	// her real name in the ordinary case, where redactStaffActorNames
	// never does.
	//
	// A Doula who has left the Practice is still named here (#1077):
	// 00111's client_portal_sees_staff reaches her staff row through the
	// Visit itself rather than through a Membership that no longer
	// exists. The LEFT JOIN in listPortalVisits stays, and so does this
	// fallback, for the one case that still has no name to reach: a
	// Doula who deleted her own login, whose name ADR-0033 overwrites on
	// the row, and whom 00111's policy therefore refuses rather than
	// showing a Client "Deleted Staff Member".
	DoulaName string `json:"doulaName"`
	// HasHappened is decided in SQL against the database's own clock, so the
	// browser never has to compare a parsed instant with its own -- the
	// difference between "Thursday at 2pm" and "she came on 18 August"
	// is a server answer, and one field carries it rather than two
	// separately-paged groups (which no cursor envelope can hold; see
	// docs/api-design.md section 4, which names Visits by name).
	HasHappened bool `json:"hasHappened"`
}

// VisitsResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type VisitsResponse struct {
	Items      []Visit `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}

// VisitsHandler lists the caller's own Engagement's scheduled and past
// Visits, furthest-future first, cursor-paginated. Must be mounted
// behind clientauth.Middleware, which has already refused (403) a
// caller who does not hold the Engagement the path names -- so, like
// DetailHandler, this reads the Engagement id off the context rather
// than off the path, and never re-decides the access question itself.
// The client-tier RLS policy (00105_visits_client_visibility.sql) is the
// second fence behind that, the same shape her Birth Plan and Contract
// reads already sit behind.
func VisitsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, ok := clientauth.Tx(r.Context())
		if !ok {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		list, err := listPortalVisits(r.Context(), tx, engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > visitPageSize
		if hasMore {
			list = list[:visitPageSize]
		}
		resp := VisitsResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.ScheduledAt, last.VisitID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// listPortalVisits is filtered by engagementID explicitly, on top of the
// RLS scoping clientauth.Middleware already set up on tx -- the app
// layer's own filter, so a bug in either one alone cannot leak rows.
//
// `scheduled_at IS NOT NULL` is the truthfulness half (#478's own
// acceptance criterion, and 00105's comment says why it is here rather
// than in the policy): `created_at` is when a Doula typed the row, so a
// Visit nobody scheduled has no instant that is true to show a Client.
//
// Ordered by (scheduled_at, id) DESC, matching message.ListHandler and
// visit.ListHandler and docs/api-design.md section 4's worked example.
// Furthest-future first is what puts every scheduled Visit ahead of
// every past one in a single stream, so the consumer groups by
// `hasHappened` with no second request and no sort of its own.
func listPortalVisits(ctx context.Context, tx *sql.Tx, engagementID string, after *pagecursor.Cursor) ([]Visit, error) {
	query := `SELECT v.id, v.scheduled_at, coalesce(s.name, $2), v.scheduled_at <= now()
		 FROM visits v
		 LEFT JOIN staff s ON s.id = v.staff_id
		 WHERE v.engagement_id = $1 AND v.scheduled_at IS NOT NULL`
	args := []any{engagementID, activity.StaffActorDisplayName}
	if after != nil {
		query += ` AND (v.scheduled_at, v.id) < ($3, $4) ORDER BY v.scheduled_at DESC, v.id DESC LIMIT $5`
		args = append(args, after.At, after.ID, visitPageSize+1)
	} else {
		query += ` ORDER BY v.scheduled_at DESC, v.id DESC LIMIT $3`
		args = append(args, visitPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("portal: list visits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []Visit{}
	for rows.Next() {
		var v Visit
		if err := rows.Scan(&v.VisitID, &v.ScheduledAt, &v.DoulaName, &v.HasHappened); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("portal: scan visit row: %w", err)
		}
		list = append(list, v)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("portal: iterate visit rows: %w", err)
	}
	return list, nil
}
