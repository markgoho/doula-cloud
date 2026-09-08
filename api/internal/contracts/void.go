package contracts

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

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
// of the Engagement's other (already-voided) Contract rows. Owner and
// Admin only, declared at the mount (contracts.Mount,
// staffauth.OwnerAndAdmin) rather than checked here -- #282, #970: every
// Doula, employee or contractor, is refused regardless of what she is
// attached to. Must be mounted through
// idempotency.Router.ExemptGated.
func PostVoidContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, values, amountCents, amountChangedAt, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if ok, refusal := TransitionVoid.Check(Status(status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
			string(StatusVoided), id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// #971: an Owner or an Admin acting on a Doula's ask is exactly
		// this transition, not a second endpoint -- any request still
		// open against this Contract is now fulfilled, so it closes with
		// the Void rather than staying open forever with nothing left to
		// grant. decline_reason stays NULL, which is what tells the
		// requester (VoidRequests on her next GET) she got a void, not a
		// decline.
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contract_void_requests
			    SET status = 'voided', decided_by = $1, decided_at = now()
			  WHERE contract_id = $2 AND status = 'open'`,
			staffID, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		voidRequests, err := listVoidRequests(r.Context(), tx, id)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		mergeFields := extractMergeFields(prose)
		out := ContractResponse{
			EngagementID:    engagementID,
			Status:          string(StatusVoided),
			Prose:           prose,
			MergeFields:     mergeFields,
			Values:          withResolvedPrice(mergeFields, values.nonEmpty(), amountCents),
			AmountChangedAt: amountChangedAt,
			VoidRequests:    voidRequests,
		}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}
