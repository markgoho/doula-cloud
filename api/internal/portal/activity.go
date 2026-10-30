package portal

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/pagecursor"
)

// activityPageSize matches engagement.activityPageSize's own reasoning --
// there is nothing portal-specific about how many rows one page carries.
const activityPageSize = 30

// staffingActions is the exclusion list built once from
// activity.StaffingActions() -- GLOSSARY.md's Activity entry: a
// Client reads her own Activity, "never who inside the Practice did
// what." An Offer names which Doula was asked, accepted or bumped, and a
// Visit reassignment names which Doula covers it; both are Practice
// roster facts, not facts about her. A void request and its refusal are
// the same sentence in its other framing -- the Practice deliberating
// with itself about her Contract, nothing of hers changed either way
// (#1096); the void itself is not in the set and still reaches her. It
// reaches the query as bound parameters (activitypage.Query's own doc
// comment), so it is a list of names here and never a fragment of SQL.
var staffingActions = buildStaffingActions()

func buildStaffingActions() []string {
	return actionNames(activity.StaffingActions())
}

// withoutBirthPlan is staffingActions with activity.BirthPlanActions()
// added: the exclusion list for an Engagement that offers her no Birth
// Plan (#2219, #2277). Built once like staffingActions, and chosen per
// read from engagement.OffersBirthPlan -- the same derivation the portal
// nav, the hub card and the Birth Plan route already ask -- so nothing
// is stored to hide the rows and correcting an outcome back to a live
// birth brings them back. The rows stay in activity, and every
// Staff-side ledger still reads them; only this read leaves them out.
// They leave in the query, not after it, so a page is still a full page
// and the cursor still finds the rest.
var withoutBirthPlan = slices.Concat(staffingActions, actionNames(activity.BirthPlanActions()))

func actionNames(actions []activity.EngagementAction) []string {
	names := make([]string, len(actions))
	for i, a := range actions {
		names[i] = string(a)
	}
	return names
}

// redactStaffActorNames applies activity.StaffActorDisplayName to every
// Staff-actor row, once, in the one reader a Client-portal caller ever
// reaches -- rather than trusting every future caller of
// activityfeed.ListForSubject (a Staff-side one included, where the real
// name is exactly what ADR-0022 asks the ledger to carry) to remember
// this redaction itself. GLOSSARY.md's Activity entry, the second half of
// the same sentence staffingActions answers: "she reads her own
// Activity ... never who inside the Practice did what." Excluding
// staffing-shaped actions (offer_*, visit_reassigned) is not enough on
// its own: every remaining Staff-authored row (a Contract sent, an
// Invoice raised) still names the individual Doula who did it unless that
// name is replaced here. A Client actor's own name and
// activity.SystemActorName ("DoulaCloud") are untouched -- the first is
// her own act, the second already reads as the product, not a person.
func redactStaffActorNames(items []activityfeed.Entry) {
	for i := range items {
		if items[i].ActorKind == "staff" {
			items[i].ActorName = activity.StaffActorDisplayName
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
// filter further, and GLOSSARY.md's Activity entry says she reads "her
// money" in full, unlike an employed Doula under ADR-0008's money tier.
// staffingActions excludes Practice-roster actions outright, and
// redactStaffActorNames replaces every surviving Staff actor's own name --
// together the two halves of that same GLOSSARY.md sentence. Must be
// mounted behind clientauth.Middleware.
//
// A Membership row never belongs here, decided rather than inherited
// (#1148, which put those rows on the Practice's own feed): the roster is
// the far side of the same GLOSSARY.md sentence -- she reads her own
// Activity, "never who inside the Practice did what" -- and who joined,
// whose roles moved, whose employment type changed and who left is that
// sentence exactly. It is already structurally impossible, since this is
// the only Client-facing reader and it names activity.SubjectEngagement,
// so nothing enforces it beyond that; the reason is written here so the
// next person to widen this call knows it was a decision and not an
// omission.
func ActivityHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, ok := clientauth.Tx(r.Context())
		if !ok {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		practiceID, birthPlan, err := engagementPracticeID(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed this row exists
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// activity's own RLS policy compares against
		// app.current_practice_id, which a Client-portal transaction never
		// sets (clientauth.Middleware sets app.current_client_id instead).
		// This is a read, not a call to activity.Record, so it sets the
		// session variable itself here -- the same way
		// payments.resolveInvoiceForEvent does for its own webhook path.
		// Nothing but this read runs on tx afterward.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		excluded := staffingActions
		if !engagement.OffersBirthPlan(birthPlan) {
			excluded = withoutBirthPlan
		}
		resp, err := activityfeed.ListForSubject(r.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, excluded, after, activityPageSize)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		redactStaffActorNames(resp.Items)

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// engagementPracticeID reads engagementID's own practice_id, and the two
// facts engagement.OffersBirthPlan asks of it -- clientauth.Middleware
// already confirmed this Engagement belongs to the caller's Client before
// this handler runs (setClientAndCheckEngagement,
// clientauth/middleware.go), so this is a lookup, not a second access
// check. The kind and the birth outcome are read only to answer that
// question and never reach the response: ADR-0015 keeps both staff-only.
func engagementPracticeID(ctx context.Context, tx *sql.Tx, engagementID string) (string, engagement.BirthPlanInputs, error) {
	var practiceID, kind string
	var birthOutcome *string
	if err := tx.QueryRowContext(ctx,
		`SELECT practice_id, kind::text, birth_outcome::text FROM engagements WHERE id = $1`,
		engagementID).Scan(&practiceID, &kind, &birthOutcome); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed this row exists
		return "", engagement.BirthPlanInputs{}, fmt.Errorf("portal: read engagement practice id: %w", err)
	}
	return practiceID, engagement.BirthPlanInputs{Kind: engagement.Kind(kind), BirthOutcome: birthOutcome}, nil
}
