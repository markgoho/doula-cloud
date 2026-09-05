package contracts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

const statusVoided = "voided"

// PostVoidContractHandler transitions the Contract for :engagementId from
// 'signed' to 'voided' -- the only transition it permits; any other
// current status (including an already-voided Contract) 409s. Voiding is
// a one-way, terminal transition: it never touches
// signed_pdf_object_path or the underlying GCS object, so the Signed PDF
// (#71) is preserved unchanged as the historical record of what was
// originally signed. Staff create a fresh Draft Contract afterward via
// PostContractHandler to capture updated terms -- there is no
// amendment/addendum entity. That recreate is only possible because
// voiding frees the Engagement's slot in contracts_engagement_id_active_key
// (00020_contracts_recreate_after_void.sql), the partial unique index
// that replaced the original table-wide UNIQUE (engagement_id); Void
// itself doesn't touch that index, but every UPDATE here targets the
// fetched row's id rather than engagement_id, so it can never affect any
// of the Engagement's other (already-voided) Contract rows. Must be
// mounted behind staffauth.Middleware.
func PostVoidContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if status != statusSigned {
			apierr.WriteError(w, "contract is not signed", http.StatusConflict)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
			statusVoided, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractVoided),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       statusVoided,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
