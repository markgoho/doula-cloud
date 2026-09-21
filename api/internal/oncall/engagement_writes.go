package oncall

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// The refusals the two per-Engagement writes own.
const (
	MsgNotABirth      = "Only a birth engagement has an on-call window."
	MsgNotAttached    = "This doula is not on this birth."
	MsgNarrowOrder    = "The last day must be on or after the first day."
	MsgNarrowDate     = "Enter a date, such as 2026-10-09."
	MsgEngagementGone = "engagement not found"
)

// RuleRequest is an Engagement's own start rule. A null StartRule clears
// the override, and the Practice's rule applies again.
type RuleRequest struct {
	StartRule *StartRule `json:"startRule"`
	StartWeek *int       `json:"startWeek"`
}

// PutRuleHandler sets or clears an Engagement's own on-call start rule,
// overriding the Practice's. Owner and Admin only, declared at the
// mount: the Practice's rule is theirs to state, and so is a departure
// from it. Recorded in the Engagement's Activity log.
//
// Must be mounted behind staffauth.Middleware.
func PutRuleHandler() http.Handler {
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
		var req RuleRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		details := map[string]string{}
		switch {
		case req.StartRule == nil && req.StartWeek != nil:
			details["startWeek"] = MsgWeekWithoutRule
		case req.StartRule == nil:
		case *req.StartRule == StartAttachmentGranted && req.StartWeek != nil:
			details["startWeek"] = MsgWeekWithoutRule
		case *req.StartRule == StartGestationalWeek:
			if req.StartWeek == nil || *req.StartWeek < minStartWeek || *req.StartWeek > maxStartWeek {
				details["startWeek"] = MsgStartWeekRange
			}
		case *req.StartRule != StartAttachmentGranted:
			details["startRule"] = MsgUnknownStartRule
		}
		if len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The on-call rule could not be saved.", details)
			return
		}

		var kind string
		var beforeRule sql.NullString
		var beforeWeek sql.NullInt64
		err := tx.QueryRowContext(r.Context(),
			`SELECT kind::text, on_call_start_rule::text, on_call_start_week
			   FROM engagements WHERE id = $1 AND practice_id = $2 FOR UPDATE`,
			engagementID, practiceID,
		).Scan(&kind, &beforeRule, &beforeWeek)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			apierr.WriteError(w, MsgEngagementGone, http.StatusNotFound)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		case kind != KindBirth:
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgNotABirth, nil)
			return
		}

		before := RuleRequest{StartWeek: intPtr(beforeWeek)}
		if beforeRule.Valid {
			rule := StartRule(beforeRule.String)
			before.StartRule = &rule
		}
		if !sameRule(before, req) {
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE engagements SET on_call_start_rule = $2, on_call_start_week = $3 WHERE id = $1`,
				engagementID, req.StartRule, req.StartWeek,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if err := recordEngagement(r.Context(), tx, practiceID, engagementID, activity.ActionOnCallRuleChanged,
				map[string]RuleRequest{diffBefore: before, diffAfter: req}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		apierr.WriteJSON(w, http.StatusOK, req)
	})
}

// NarrowingRequest is one Doula's narrowing, as calendar days in the
// Practice's zone. Both null clears it: she is on call for the whole
// window again.
type NarrowingRequest struct {
	From *string `json:"from"`
	To   *string `json:"to"`
}

// PutNarrowingHandler sets or clears one attached Doula's narrowing --
// the part of the window she is on call for, which is what makes a
// primary and a backup expressible with no second entity. Owner and
// Admin only, declared at the mount, the pair that schedules. Recorded
// in the Engagement's Activity log.
//
// Must be mounted behind staffauth.Middleware.
func PutNarrowingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		engagementID, staffID := r.PathValue("engagementId"), r.PathValue("staffId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) || !staffauth.ParseUUID(w, "staff", staffID) {
			return
		}
		var req NarrowingRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.From, req.To = blankToNil(req.From), blankToNil(req.To)
		details := map[string]string{}
		for field, value := range map[string]*string{"from": req.From, "to": req.To} {
			if value != nil {
				if _, err := time.Parse(dateLayout, *value); err != nil {
					details[field] = MsgNarrowDate
				}
			}
		}
		if len(details) == 0 && req.From != nil && req.To != nil && *req.To < *req.From {
			details["to"] = MsgNarrowOrder
		}
		if len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The on-call days could not be saved.", details)
			return
		}

		var before NarrowingRequest
		var from, to sql.NullString
		err := tx.QueryRowContext(r.Context(),
			`SELECT ea.on_call_from::text, ea.on_call_to::text
			   FROM engagement_attachments ea
			   JOIN engagements e ON e.id = ea.engagement_id
			  WHERE ea.engagement_id = $1 AND ea.staff_id = $2 AND e.practice_id = $3
			    AND ea.origin = 'granted' AND ea.ended_at IS NULL
			  FOR UPDATE OF ea`,
			engagementID, staffID, practiceID,
		).Scan(&from, &to)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, MsgNotAttached, nil)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		before.From, before.To = nullString(from), nullString(to)

		if !sameString(before.From, req.From) || !sameString(before.To, req.To) {
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE engagement_attachments SET on_call_from = $3, on_call_to = $4
				  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
				engagementID, staffID, req.From, req.To,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if err := recordEngagement(r.Context(), tx, practiceID, engagementID, activity.ActionOnCallNarrowingChanged,
				map[string]any{"staffId": staffID, diffBefore: before, diffAfter: req}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		apierr.WriteJSON(w, http.StatusOK, req)
	})
}

// recordEngagement writes one on-call action to an Engagement's ledger,
// naming the acting Staff member.
func recordEngagement(ctx context.Context, tx *sql.Tx, practiceID, engagementID string, action activity.EngagementAction, diff any) error {
	actor, _ := staffauth.StaffID(ctx)
	return record(ctx, tx, activity.SubjectEngagement, practiceID, engagementID, actor, string(action), diff)
}

func sameRule(a, b RuleRequest) bool {
	return sameString((*string)(a.StartRule), (*string)(b.StartRule)) && sameInt(a.StartWeek, b.StartWeek)
}

func sameString(a, b *string) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func sameInt(a, b *int) bool       { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func blankToNil(s *string) *string {
	if isBlank(s) {
		return nil
	}
	return s
}
