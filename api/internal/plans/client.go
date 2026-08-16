package plans

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/clientauth"
)

// birthPlanType is the only plan_type ClientGetBirthPlanHandler will ever
// query for -- hardcoded rather than taken from the URL, so Care Plan is
// structurally unreachable through this handler regardless of what
// 00013_plan_instances_client_birth_plan_read.sql's RLS policy allows.
const birthPlanType = "birth_plan"

// ClientGetBirthPlanHandler views the Birth Plan instance for the
// Client-portal caller's Engagement -- clientauth.Middleware has already
// confirmed the caller's Client owns :engagementId. Read-only: there is no
// corresponding PUT/POST route for this population. Must be mounted behind
// clientauth.Middleware.
func ClientGetBirthPlanHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		fields, answers, err := fetchInstance(r.Context(), tx, engagementID, birthPlanType)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := InstanceResponse{EngagementID: engagementID, PlanType: birthPlanType, Fields: fields, Answers: answers.nonEmpty()}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
