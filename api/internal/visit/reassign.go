package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// ReassignRequest names the Doula a Visit should be reassigned to.
type ReassignRequest struct {
	StaffID string `json:"staffId"`
}

// ReassignResponse confirms who a Visit is now assigned to.
type ReassignResponse struct {
	VisitID string `json:"visitId"`
	StaffID string `json:"staffId"`
}

// ReassignHandler reassigns a Visit's staff_id to a different Doula at the
// same Practice -- coverage/handoff is just editing that field, no
// separate coverage entity. Must be mounted behind staffauth.Middleware;
// the caller must hold the Doula role at the current Practice.
func ReassignHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireDoula(w, r)
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		visitID := r.PathValue("visitId")
		if !staffauth.ParseUUID(w, "visit", visitID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req ReassignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !staffauth.ParseUUID(w, "staff", req.StaffID) {
			return
		}

		hasMembership, isDoula, employmentType, err := doulaMembership(r.Context(), tx, practiceID, req.StaffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !hasMembership {
			http.Error(w, "staff member not found at this practice", http.StatusBadRequest)
			return
		}
		if !isDoula {
			http.Error(w, "staff member does not hold the Doula role at this practice", http.StatusBadRequest)
			return
		}
		// A contractor is put on a birth by her own acceptance of an Offer
		// and by nothing else (CONTEXT.md's Attachment entry), so handing
		// her a Visit is refused unless she already holds the attachment
		// that says she agreed.
		if employmentType != employeeType {
			attached, err := hasGrantedAttachment(r.Context(), tx, engagementID, req.StaffID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !attached {
				http.Error(w, "that contractor has not accepted an offer on this engagement", http.StatusBadRequest)
				return
			}
		}

		// engagement_id is filtered explicitly, on top of the RLS scoping
		// staffauth.Middleware already set up on tx, so a Visit can't be
		// reassigned via an engagementId/visitId pair that don't actually
		// belong together.
		result, err := tx.ExecContext(r.Context(),
			`UPDATE visits SET staff_id = $1 WHERE id = $2 AND engagement_id = $3`,
			req.StaffID, visitID, engagementID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			http.Error(w, "visit not found", http.StatusNotFound)
			return
		}

		// The employee the Visit was handed to is now on this birth, so
		// she gets a granted attachment even though she is not the actor
		// -- ADR-0008's "an Admin scheduling her onto a Visit ... that is
		// a granted attachment, written explicitly". attached_by is the
		// person who did the handing, not the person handed to. A
		// contractor needs none: the check above already proved she holds
		// the one her own acceptance opened.
		if employmentType == employeeType {
			actorStaffID, _ := staffauth.StaffID(r.Context())
			if err := staffauth.Grant(r.Context(), tx, engagementID, req.StaffID, actorStaffID, nil, nil); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(ReassignResponse{VisitID: visitID, StaffID: req.StaffID}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
