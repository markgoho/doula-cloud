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
	"doula-cloud/api/internal/activitygate"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/apierr"
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
	// Detail is one sentence describing what the diff says, already in
	// people's names -- present only for an action that has something
	// to add beyond its own name, absent for every other one, so a
	// reader falls back to the generic description it renders today.
	// Today only visit_reassigned carries it (#887).
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// ActivityListResponse is docs/api-design.md section 4's envelope.
type ActivityListResponse struct {
	Items      []ActivityEntry `json:"items"`
	NextCursor *string         `json:"nextCursor,omitempty"`
	HasMore    bool            `json:"hasMore"`
}

// moneyActionsNotIn is the SQL-literal form of
// activitygate.RestrictedActions(activity.SubjectEngagement), built once
// from that single source of truth rather than hand-copied, so the write
// side's action names and this read filter can't drift apart. Every
// value is a compile-time constant this package itself wrote (never
// request input), so building it into the query text carries no
// injection risk.
var moneyActionsNotIn = buildMoneyActionsNotIn()

func buildMoneyActionsNotIn() string {
	actions := activitygate.RestrictedActions(activity.SubjectEngagement)
	quoted := make([]string, len(actions))
	for i, a := range actions {
		quoted[i] = "'" + a + "'"
	}
	return strings.Join(quoted, ", ")
}

// ListActivityHandler lists an Engagement's activity entries, most recent
// first, cursor-paginated -- ADR-0022's ledger, read through the shared
// activitygate.CanAccessSubject gate (#485), which reuses the same
// reader.CanAccessEngagement check visit.ListHandler and DetailHandler
// apply: 404s a contractor with no open, granted attachment exactly as
// they do. The money filter beneath that gate is ADR-0008's read table as
// amended by #282, via activitygate.Bypasses/RestrictedActions: Owner,
// Admin and an employed Doula see every entry; only a contractor never
// sees the Practice's Contract price (contract_priced,
// contract_amount_overridden, contract_amount_repriced) or an
// Invoice/payment entry -- the Contract entity's own lifecycle
// (contract_created, contract_sent, contract_signed, contract_voided,
// contract_void_requested, contract_void_declined) carries no price and
// reaches her the same as anyone else (#972). Applied as a SQL predicate
// so no row it excludes ever leaves the database. Must be mounted behind
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

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := activitygate.CanAccessSubject(r.Context(), tx, reader, activity.SubjectEngagement, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		moneyGate := activitygate.Bypasses(reader)

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		page, err := listEngagementActivity(r.Context(), tx, practiceID, engagementID, moneyGate, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ActivityListResponse{
			Items:      page.Items,
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		})
	})
}

// activityProjection is this package's own half of a subject-scoped
// activity row (#1150): the diff ADR-0022's ledger is built around, and
// the two staff names a visit_reassigned entry's sentence needs.
// activitypage owns everything else -- the Practice and subject scoping,
// the newest-first (created_at, id) cursor, the sentinel row, the page
// trim, and the actor's own resolved name, which used to be spelled
// here and rendered a departed colleague as an empty cell.
//
// The two staff joins resolve a visit_reassigned entry's two ids to
// names the same way the shared actor join resolves actor_staff_id,
// keyed on activity's own DiffKeyAssignedStaffID* constants so the write
// side names those keys once (#887). Neither the joins nor the columns
// are request input: every interpolated value is a compile-time constant
// this package itself wrote.
var activityProjection = activitypage.Projection[ActivityEntry]{
	Joins: []string{
		fmt.Sprintf(`LEFT JOIN staff before_staff ON before_staff.id = (a.diff ->> '%s')::uuid`, activity.DiffKeyAssignedStaffIDBefore), //nolint:gosec // a package-internal constant, not request input
		fmt.Sprintf(`LEFT JOIN staff after_staff  ON after_staff.id  = (a.diff ->> '%s')::uuid`, activity.DiffKeyAssignedStaffIDAfter),  //nolint:gosec // a package-internal constant, not request input
	},
	Columns: []string{"a.diff", "before_staff.name", "after_staff.name"},
	Row: func() ([]any, func(activitypage.Row) ActivityEntry) {
		var diff []byte
		var beforeStaffName, afterStaffName sql.NullString
		return []any{&diff, &beforeStaffName, &afterStaffName}, func(r activitypage.Row) ActivityEntry {
			return ActivityEntry{
				Action:    r.Action,
				Diff:      diff,
				ActorKind: r.ActorKind,
				ActorName: r.ActorName,
				Detail:    reassignmentDetail(r.Action, beforeStaffName, afterStaffName),
				CreatedAt: r.CreatedAt,
			}
		}
	},
}

// activityQuery is the page this handler asks for, given who is reading.
//
// moneyGate is ADR-0008's read table as amended by #282: an Owner, an
// Admin or an employed Doula sees every entry, so that query carries no
// exclusion predicate at all; only a contractor's does. Before #1150 the
// gate was a bool parameter and one query text served both, so the
// predicate sat in the plan even for the readers it never excludes
// anything from. Two texts, each built from this package's own
// constants, are the same filter stated once per audience -- and the
// contractor's is the heavier plan, which is the one the JIT canary
// beside this file now explains.
func activityQuery(practiceID, engagementID string, moneyGate bool, after *pagecursor.Cursor) activitypage.Query {
	q := activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		After:       after,
		PageSize:    activityPageSize,
	}
	if !moneyGate {
		q.ExcludedActions = moneyActionsNotIn
	}
	return q
}

// listEngagementActivity reads one page of engagementID's activity
// through the shared subject reader, filtered by practiceID (on top of
// the RLS scoping staffauth.Middleware already set up on tx -- the app
// layer's own filter, so a bug in either alone can't leak rows) and
// subject_kind = 'engagement'. Every row comes back with the actor's
// name already resolved, never a bare id a reader has to look up itself.
func listEngagementActivity(ctx context.Context, tx *sql.Tx, practiceID, engagementID string, moneyGate bool, after *pagecursor.Cursor) (activitypage.Page[ActivityEntry], error) {
	page, err := activitypage.List(ctx, tx, activityQuery(practiceID, engagementID, moneyGate, after), activityProjection)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return activitypage.Page[ActivityEntry]{}, fmt.Errorf("engagement: list activity: %w", err)
	}
	return page, nil
}

// reassignmentDetail renders a visit_reassigned entry as the move it
// records, in the two people's names -- the ids in the diff resolved by
// the query's own LEFT JOINs, the same way actorName is resolved, so no
// reader has to look either of them up. Every other action returns "",
// which the JSON omits: those entries render through the generic
// description the app already gives them.
func reassignmentDetail(action string, before, after sql.NullString) string {
	if action != string(activity.ActionVisitReassigned) {
		return ""
	}
	return fmt.Sprintf("Visit reassigned from %s to %s", staffDisplayName(before), staffDisplayName(after))
}

func staffDisplayName(name sql.NullString) string {
	if !name.Valid {
		return activity.DepartedStaffName
	}
	return name.String
}
