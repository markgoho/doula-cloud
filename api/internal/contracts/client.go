package contracts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/clientauth"
)

// ClientGetContractHandler views the Contract for the Client-portal
// caller's Engagement -- read-only, no PUT/POST route for this
// population; the signing flow itself is #70. clientauth.Middleware has
// already confirmed the caller's Client owns :engagementId; Postgres RLS
// (contracts_client_visibility, 00017_contracts_client_visibility.sql) is
// what actually keeps a Draft Contract unreachable here -- fetchContract
// finds no row and this handler 404s, the same "zero rows, not an
// error" shape plans.ClientGetBirthPlanHandler relies on. Must be
// mounted behind clientauth.Middleware.
func ClientGetContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       status,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
