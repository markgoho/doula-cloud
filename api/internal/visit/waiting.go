package visit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/staffauth"
)

const waitingPageSize = 30

const waitingForeverYears = 100

// WaitingEngagement is one Engagement holding Visits with no date that wait
// for a new Doula (#2874), and how many.
type WaitingEngagement struct {
	EngagementID string `json:"engagementId"`
	ClientName   string `json:"clientName"`
	UndatedCount int    `json:"undatedCount"`
}

// WaitingVisitsResponse answers "what is waiting for a new Doula" for the
// whole Practice, dated and undated alike (#2874). The counts are the whole
// Practice's, whatever page Items is on. Items are the Engagements holding an
// undated Visit, the ones the Schedule cannot show, by Client name. DatedCount is the Schedule's own
// "Needing a new Doula" count (#2863) over every day from now on, by the same
// predicate; LatestScheduledAt is the last of those Visits, so a link to the
// Schedule can name days that hold them all.
type WaitingVisitsResponse struct {
	Total             int                 `json:"total"`
	DatedCount        int                 `json:"datedCount"`
	UndatedCount      int                 `json:"undatedCount"`
	LatestScheduledAt *time.Time          `json:"latestScheduledAt,omitempty"`
	Items             []WaitingEngagement `json:"items"`
	NextCursor        *string             `json:"nextCursor,omitempty"`
	HasMore           bool                `json:"hasMore"`
}

type waitingAfter struct {
	name string
	id   string
}

func decodeWaitingCursor(s string) (waitingAfter, error) {
	c, err := pagecursor.DecodeKeyed(s, orderSoonest)
	if err != nil {
		return waitingAfter{}, fmt.Errorf("visit: waiting cursor: %w", err)
	}
	if len(c.Keys) != 1 {
		return waitingAfter{}, errors.New("visit: waiting cursor: wrong key count")
	}
	return waitingAfter{name: c.Keys[0], id: c.ID}, nil
}

type waitingRow struct {
	WaitingEngagement
	sortName string
}

// WaitingVisitsHandler lists the Engagements whose Visits wait for a new
// Doula, with the Practice-wide counts (#2874). The Schedule lists only
// scheduled Visits, so an undated Visit whose Doula was taken off its
// Engagement can be found nowhere but here and on its own Engagement.
//
// Owner and Admin only: reassigning is theirs (ADR-0006), and a Doula has no
// use for a count of what other Doulas left. A read, so nothing is written and
// the audit trail has nothing to record. Must be mounted behind
// staffauth.Middleware.
func WaitingVisitsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		ctx := r.Context()
		now := clock.Now(ctx)
		page, ok := pagecursor.FetchKeyed(w, r, MsgInvalidCursor, orderSoonest, waitingPageSize,
			decodeWaitingCursor,
			func(after *waitingAfter) ([]waitingRow, error) {
				return listWaitingEngagements(ctx, c.Tx, c.PracticeID, now, after)
			},
			func(row waitingRow) ([]string, string) { return []string{row.sortName}, row.EngagementID })
		if !ok {
			return
		}

		resp := WaitingVisitsResponse{HasMore: page.HasMore, NextCursor: page.Next, Items: make([]WaitingEngagement, 0, len(page.Items))}
		for _, row := range page.Items {
			resp.Items = append(resp.Items, row.WaitingEngagement)
		}
		dated, err := countScheduledVisits(ctx, c.Tx, c.PracticeID, scheduleFilter{
			from: now, to: now.AddDate(waitingForeverYears, 0, 0), offEngagement: true, zone: time.UTC,
		})
		if err == nil {
			err = c.Tx.QueryRowContext(ctx, `SELECT count(*)
				  FROM visits v JOIN engagements e ON e.id = v.engagement_id
				 WHERE e.practice_id = $1 AND v.scheduled_at IS NULL
				   AND `+offEngagementBefore+`$2`+offEngagementAfter,
				c.PracticeID, now).Scan(&resp.UndatedCount)
		}
		if err == nil {
			err = c.Tx.QueryRowContext(ctx, `SELECT max(v.scheduled_at)
				  FROM visits v JOIN engagements e ON e.id = v.engagement_id
				 WHERE e.practice_id = $1 AND v.scheduled_at IS NOT NULL
				   AND `+offEngagementBefore+`$2`+offEngagementAfter,
				c.PracticeID, now).Scan(&resp.LatestScheduledAt)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp.DatedCount = dated
		resp.Total = dated + resp.UndatedCount
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

const waitingNameKey = `lower(concat_ws(' ', COALESCE(NULLIF(cl.preferred_name, ''), cl.given_name), NULLIF(cl.family_name, '')))`

func listWaitingEngagements(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time, after *waitingAfter) ([]waitingRow, error) {
	var afterName, afterID any
	if after != nil {
		afterName, afterID = after.name, after.id
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id, cl.given_name, cl.preferred_name, cl.family_name, `+waitingNameKey+` AS sort_name,
	       count(*)
	  FROM visits v
	  JOIN engagements e ON e.id = v.engagement_id
	  JOIN clients cl ON cl.id = e.client_id
	 WHERE e.practice_id = $1 AND v.scheduled_at IS NULL
	   AND `+offEngagementBefore+`$2`+offEngagementAfter+`
	   AND ($3::text IS NULL OR (`+waitingNameKey+`, e.id) > ($3::text, $4::uuid))
	 GROUP BY e.id, cl.given_name, cl.preferred_name, cl.family_name
	 ORDER BY sort_name, e.id
	 LIMIT $5`, practiceID, now, afterName, afterID, waitingPageSize+1)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: list waiting engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []waitingRow{}
	for rows.Next() {
		var row waitingRow
		var givenName string
		var preferredName, familyName sql.NullString
		if err := rows.Scan(&row.EngagementID, &givenName, &preferredName, &familyName, &row.sortName,
			&row.UndatedCount); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan waiting engagement: %w", err)
		}
		row.ClientName = personname.Full(givenName, preferredName.String, familyName.String)
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate waiting engagements: %w", err)
	}
	return list, nil
}
