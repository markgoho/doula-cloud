package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/pagecursor"
)

// activityPageSize matches engagement.activityPageSize's own reasoning --
// there is nothing portal-specific about how many rows one page carries.
const activityPageSize = 30

// staffingActionsNotIn is the SQL-literal exclusion clause built once
// from activity.StaffingActions() -- CONTEXT.md's Activity entry: a
// Client reads her own Activity, "never who inside the Practice did
// what." An Offer names which Doula was asked, accepted or bumped, and a
// Visit reassignment names which Doula covers it; both are Practice
// roster facts, not facts about her. Every value here is a compile-time
// constant this package itself wrote, never request input, matching
// engagement.moneyActionsNotIn's own reasoning.
var staffingActionsNotIn = buildStaffingActionsNotIn()

func buildStaffingActionsNotIn() string {
	actions := activity.StaffingActions()
	quoted := make([]string, len(actions))
	for i, a := range actions {
		quoted[i] = "'" + string(a) + "'"
	}
	return strings.Join(quoted, ", ")
}

// staffActorDisplayName is what a Staff actor's row renders as on a
// Client's own ledger -- CONTEXT.md's Activity entry, the second half of
// the same sentence staffingActionsNotIn answers: "she reads her own
// Activity ... never who inside the Practice did what." Excluding
// staffing-shaped actions (offer_*, visit_reassigned) is not enough on
// its own: every remaining Staff-authored row (a Contract sent, an
// Invoice raised) still names the individual Doula who did it unless that
// name is replaced here. A Client actor's own name and
// activity.SystemActorName ("Doula Cloud") are untouched -- the first is
// her own act, the second already reads as the product, not a person.
const staffActorDisplayName = "Your practice"

// redactStaffActorNames applies staffActorDisplayName to every Staff-actor
// row, once, in the one reader a Client-portal caller ever reaches --
// rather than trusting every future caller of activityfeed.ListForSubject
// (a Staff-side one included, where the real name is exactly what ADR-0022
// asks the ledger to carry) to remember this redaction itself.
func redactStaffActorNames(items []activityfeed.Entry) {
	for i := range items {
		if items[i].ActorKind == "staff" {
			items[i].ActorName = staffActorDisplayName
		}
	}
}

// ActivityHandler lists the caller's own Engagement's activity, newest
// first, cursor-paginated -- #486 AC4/AC5's record-scoped read for the
// Client portal, via activityfeed.ListForSubject. It applies no
// activitygate check (that gate has no Reader for a Client-portal caller
// at all): clientauth.Middleware has already confirmed the caller's own
// Client owns this Engagement before this handler ever runs, which is
// the whole of the access decision -- there is no role hierarchy to
// filter further, and CONTEXT.md's Activity entry says she reads "her
// money" in full, unlike an employed Doula under ADR-0008's money tier.
// staffingActionsNotIn excludes Practice-roster actions outright, and
// redactStaffActorNames replaces every surviving Staff actor's own name --
// together the two halves of that same CONTEXT.md sentence. Must be
// mounted behind clientauth.Middleware.
func ActivityHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, ok := clientauth.Tx(r.Context())
		if !ok {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		practiceID, err := engagementPracticeID(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed this row exists
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// activity's own RLS policy compares against
		// app.current_practice_id, which a Client-portal transaction never
		// sets (clientauth.Middleware sets app.current_client_id instead) --
		// see activity.ScopeToPractice's own doc comment for the landmine
		// this avoids. Nothing but this read runs on tx afterward.
		if err := activity.ScopeToPractice(r.Context(), tx, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		resp, err := activityfeed.ListForSubject(r.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, staffingActionsNotIn, after, activityPageSize)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		redactStaffActorNames(resp.Items)

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// engagementPracticeID reads engagementID's own practice_id --
// clientauth.Middleware already confirmed this Engagement belongs to the
// caller's Client before this handler runs (setClientAndCheckEngagement,
// clientauth/middleware.go), so this is a lookup, not a second access
// check.
func engagementPracticeID(ctx context.Context, tx *sql.Tx, engagementID string) (string, error) {
	var practiceID string
	// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed this row exists
	if err := tx.QueryRowContext(ctx, `SELECT practice_id FROM engagements WHERE id = $1`, engagementID).Scan(&practiceID); err != nil {
		return "", fmt.Errorf("portal: read engagement practice id: %w", err)
	}
	return practiceID, nil
}
