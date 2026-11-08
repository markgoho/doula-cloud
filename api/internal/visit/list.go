package visit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
)

// pageSize is the fixed number of Visits returned per page, matching
// message.pageSize's reasoning.
const pageSize = 30

// Visit is one row of a Visit list: who it's assigned to, when it was
// created, when it is scheduled (#250), its own free-text notes (#251)
// -- both nullable, since a Visit may carry no scheduled instant and no
// notes have ever been written against it -- and its derived type
// (#281). Staff-only: no Client-facing read path exists for a Visit at
// all (#251's own AC), so there is nothing to gate Notes or Type out of
// yet -- Type inherits this same gate rather than a rule of its own,
// since nothing about it is more sensitive than the Visit it describes.
type Visit struct {
	VisitID     string     `json:"visitId"`
	StaffID     string     `json:"staffId"`
	StaffName   string     `json:"staffName"`
	CreatedAt   time.Time  `json:"createdAt"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	// Type is DeriveType's own output (type.go): prenatal, birth or
	// postpartum. Never stored, never written -- listVisits computes it
	// fresh from this row's own instant and the Engagement's
	// pregnancy_ended_on on every read, so correcting that date retypes
	// every Visit at once with no separate write and no backfill. The
	// calendar day each instant falls on is the Practice's own, not
	// UTC's and not the reader's (#953).
	Type string `json:"type"`
	// IsOffEngagement is true for a Visit not yet happened (future or undated) whose Doula has been taken
	// off the Engagement (#2596, off_engagement.go).
	IsOffEngagement bool `json:"isOffEngagement"`
}

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type ListResponse struct {
	Items      []Visit `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}

// ListHandler lists Visits under an Engagement, most recent first,
// cursor-paginated, regardless of which Doula it's assigned to -- same
// "any Staff at the Practice can see it" visibility as
// engagement.ListHandler, narrowed the same way by ADR-0008's attachment
// rule for a contractor Doula. Must be mounted behind
// staffauth.Middleware.
//
// Ordering flipped from ascending (oldest first) to (created_at, id) DESC
// -- #446, matching docs/api-design.md section 4's own worked example,
// which is literally this query. The frontend reverses a page for
// display, the same pattern message.ListHandler already established.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, engagementID, ok := staffauth.RequireEngagementReach(w, r, "engagement not found")
		if !ok {
			return
		}

		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if !ok {
			return
		}

		// Read once for the whole page, before the rows: every Visit on
		// it types against the same Practice's day (#953). The zone comes
		// from practicetimezone, which owns it for every calendar-day
		// comparison in the BFF rather than only for this one (#1166).
		zone, err := practicetimezone.Load(r.Context(), tx, practiceID)
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		list, err := listVisits(r.Context(), tx, engagementID, after, zone)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		list, hasMore := pagecursor.Trim(list, pageSize)
		resp := ListResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.CreatedAt, last.VisitID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// listVisits is filtered by engagementID explicitly, on top of the RLS
// scoping staffauth.Middleware already set up on tx -- the app layer's own
// filter, so a bug in either one alone can't leak rows.
//
// Joins engagements for pregnancy_ended_on -- #281's own pivot -- rather
// than a second query per row: one Engagement backs every row this
// query returns, so the join adds one lookup for the whole page, not
// one per Visit. Read as ::text, matching outcome.go's own convention,
// so DeriveType compares two YYYY-MM-DD strings rather than a
// time.Time whose zone would have to be guessed.
//
// staff is a LEFT JOIN, not the INNER JOIN it used to be (#1322):
// staff_practice_visibility (00002) reaches a staff row only through a
// live practice_memberships row, so once the assigned Doula's own
// Membership ends -- a plain departure or ADR-0033's login deletion,
// both a hard DELETE FROM practice_memberships -- an INNER JOIN drops
// the Visit from the whole result set, not just her name from it, which
// is the audit-trail gap CLAUDE.md names ("who did it and when" must
// stay answerable). StaffID is read from v.staff_id, which always
// exists, rather than from s.id, which does not once the join finds no
// row; StaffName falls back to activity.DepartedStaffName, the same
// word #887/#1150 settled on for the Activity ledger and #1322's
// message.listMessages now also uses.
//
// zone is the Practice's own timezone, loaded once by the caller (#953):
// it decides which calendar day each row's instant falls on, and every
// row on the page uses the same one.
func listVisits(ctx context.Context, tx *sql.Tx, engagementID string, after *pagecursor.Cursor, zone *time.Location) ([]Visit, error) {
	query := `SELECT v.id, v.staff_id, s.name, v.created_at, v.scheduled_at, v.notes, e.pregnancy_ended_on::text,
		        ` + offEngagementBefore + `$2` + offEngagementAfter + `
		 FROM visits v
		 LEFT JOIN staff s ON s.id = v.staff_id
		 JOIN engagements e ON e.id = v.engagement_id
		 WHERE v.engagement_id = $1`
	args := []any{engagementID, clock.Now(ctx)}
	if after != nil {
		query += ` AND (v.created_at, v.id) < ($3, $4) ORDER BY v.created_at DESC, v.id DESC LIMIT $5`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY v.created_at DESC, v.id DESC LIMIT $3`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("visit: list visits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []Visit{}
	for rows.Next() {
		var v Visit
		var staffName sql.NullString
		var scheduledAt sql.NullTime
		var notes sql.NullString
		var pregnancyEndedOn sql.NullString
		if err := rows.Scan(&v.VisitID, &v.StaffID, &staffName, &v.CreatedAt, &scheduledAt, &notes, &pregnancyEndedOn, &v.IsOffEngagement); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan visit row: %w", err)
		}
		if staffName.Valid {
			v.StaffName = staffName.String
		} else {
			v.StaffName = activity.DepartedStaffName
		}
		if scheduledAt.Valid {
			v.ScheduledAt = &scheduledAt.Time
		}
		if notes.Valid {
			v.Notes = &notes.String
		}
		// #281: the Visit's own scheduled instant when it has one,
		// otherwise when it was logged -- the coalesce DeriveType's own
		// doc comment leaves to the caller.
		at := v.CreatedAt
		if v.ScheduledAt != nil {
			at = *v.ScheduledAt
		}
		var endedOn *string
		if pregnancyEndedOn.Valid {
			endedOn = &pregnancyEndedOn.String
		}
		v.Type = DeriveType(at, endedOn, zone)
		list = append(list, v)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate visit rows: %w", err)
	}
	return list, nil
}
