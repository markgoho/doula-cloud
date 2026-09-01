package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// activityPageSize is the fixed number of activity rows one page carries,
// matching visit.pageSize's reasoning.
const activityPageSize = 30

// ActivityEntry is one row of an Engagement's activity ledger (ADR-0022):
// what happened, the diff, who did it, and when. ActorName is always
// populated, never a bare id a reader has to resolve itself -- "Doula
// Cloud" for a system actor (ADR-0022: never "System"), the acting Staff
// member's name, or the acting Client's preferred name.
type ActivityEntry struct {
	Action    string          `json:"action"`
	Diff      json.RawMessage `json:"diff"`
	ActorKind string          `json:"actorKind"`
	ActorName string          `json:"actorName"`
	CreatedAt time.Time       `json:"createdAt"`
}

// ActivityListResponse is docs/api-design.md section 4's envelope.
type ActivityListResponse struct {
	Items      []ActivityEntry `json:"items"`
	NextCursor *string         `json:"nextCursor,omitempty"`
	HasMore    bool            `json:"hasMore"`
}

// moneyActionsNotIn is the SQL-literal form of activity.MoneyActions(),
// built once from that single source of truth rather than hand-copied,
// so the write side's action names and this read filter can't drift
// apart. Every value is a compile-time constant this package itself
// wrote (never request input), so building it into the query text
// carries no injection risk.
var moneyActionsNotIn = buildMoneyActionsNotIn()

func buildMoneyActionsNotIn() string {
	actions := activity.MoneyActions()
	quoted := make([]string, len(actions))
	for i, a := range actions {
		quoted[i] = "'" + string(a) + "'"
	}
	return strings.Join(quoted, ", ")
}

// ListActivityHandler lists an Engagement's activity entries, most recent
// first, cursor-paginated -- ADR-0022's ledger, read through the same
// gate as the Engagement itself (reader.CanAccessEngagement, matching
// visit.ListHandler and DetailHandler: 404s a contractor with no open
// attachment exactly as they do). The money filter beneath that gate is
// ADR-0008's read table: Owner and Admin see every entry; anyone else --
// an employed Doula or a contractor alike -- never sees a Contract-price
// or Invoice/payment entry, applied as a SQL predicate so no row it
// excludes ever leaves the database. Must be mounted behind
// staffauth.Middleware.
func ListActivityHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
		}
		moneyGate := reader.Has("owner") || reader.Has("admin")

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		rows, err := listEngagementActivity(r.Context(), tx, practiceID, engagementID, moneyGate, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(rows) > activityPageSize
		if hasMore {
			rows = rows[:activityPageSize]
		}
		items := make([]ActivityEntry, len(rows))
		for i, row := range rows {
			items[i] = row.ActivityEntry
		}
		resp := ActivityListResponse{Items: items, HasMore: hasMore}
		if hasMore {
			last := rows[len(rows)-1]
			next := pagecursor.Encode(last.CreatedAt, last.cursorID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// listEngagementActivityQueryTemplate and listEngagementActivityAfterQueryTemplate
// differ only in the cursor's WHERE clause and the LIMIT placeholder, the
// shape message.listMessages and visit.listVisits already use. moneyGate
// ($2) short-circuits the action exclusion for Owner/Admin so the query
// never evaluates it for the common all-access case. %s stands in for
// activity.SubjectEngagement and moneyActionsNotIn -- both compile-time
// constants this package itself wrote (see moneyActionsNotIn's own doc
// comment), never request input, so gosec's G201 (SQL query built from a
// format string) does not apply to the fmt.Sprintf calls below.
const listEngagementActivityQueryTemplate = `SELECT a.id, a.action, a.diff, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	WHERE a.practice_id = $1 AND a.subject_kind = '%[1]s' AND a.subject_id = $2
	  AND ($3 OR a.action NOT IN (%[2]s))
	ORDER BY a.created_at DESC, a.id DESC LIMIT $4`

const listEngagementActivityAfterQueryTemplate = `SELECT a.id, a.action, a.diff, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	WHERE a.practice_id = $1 AND a.subject_kind = '%[1]s' AND a.subject_id = $2
	  AND ($3 OR a.action NOT IN (%[2]s))
	  AND (a.created_at, a.id) < ($4, $5)
	ORDER BY a.created_at DESC, a.id DESC LIMIT $6`

var listEngagementActivityQuery = fmt.Sprintf(listEngagementActivityQueryTemplate, activity.SubjectEngagement, moneyActionsNotIn) //nolint:gosec // both interpolated values are package-internal constants, not request input

var listEngagementActivityAfterQuery = fmt.Sprintf(listEngagementActivityAfterQueryTemplate, activity.SubjectEngagement, moneyActionsNotIn) //nolint:gosec // both interpolated values are package-internal constants, not request input

// activityRow is ActivityEntry plus the row id ListActivityHandler needs
// to mint a cursor but never puts in the response -- activity rows have
// no id in the public shape, the same way visit.Visit's cursor field
// (VisitID) doubles as both.
type activityRow struct {
	ActivityEntry
	cursorID string
}

// listEngagementActivity reads one page of engagementID's activity,
// filtered by practiceID (on top of the RLS scoping staffauth.Middleware
// already set up on tx -- the app layer's own filter, so a bug in either
// alone can't leak rows) and subject_kind = 'engagement'. actorName is
// resolved here rather than left to the caller: a staff actor's name, a
// client actor's PreferredName, or activity.SystemActorName ("Doula
// Cloud", never "System" -- ADR-0022) for a system actor, so every row
// this returns already carries a name a reader never has to look up
// itself.
func listEngagementActivity(ctx context.Context, tx *sql.Tx, practiceID, engagementID string, moneyGate bool, after *pagecursor.Cursor) ([]activityRow, error) {
	var rows *sql.Rows
	var err error
	if after == nil {
		rows, err = tx.QueryContext(ctx, listEngagementActivityQuery, practiceID, engagementID, moneyGate, activityPageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, listEngagementActivityAfterQuery,
			practiceID, engagementID, moneyGate, after.At, after.ID, activityPageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("engagement: list activity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []activityRow{}
	for rows.Next() {
		var row activityRow
		var diff []byte
		var actorKind string
		var staffName, clientGivenName, clientPreferredName sql.NullString
		if err := rows.Scan(&row.cursorID, &row.Action, &diff, &actorKind,
			&staffName, &clientGivenName, &clientPreferredName, &row.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("engagement: scan activity row: %w", err)
		}
		row.Diff = diff
		row.ActorKind = actorKind
		switch actorKind {
		case "staff":
			row.ActorName = staffName.String
		case "client":
			row.ActorName = client.PreferredName(clientGivenName.String, clientPreferredName.String)
		default:
			row.ActorName = activity.SystemActorName
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: iterate activity rows: %w", err)
	}
	return items, nil
}
