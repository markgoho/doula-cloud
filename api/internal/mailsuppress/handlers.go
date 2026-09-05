package mailsuppress

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// suppressionDTO is one row of the Practice's suppressed-address list.
// Cause is carried rather than a ready-made sentence because it also
// decides whether the row offers a Clear at all, and that is a fact
// about the suppression, not a label.
type suppressionDTO struct {
	Address   string    `json:"address"`
	Cause     string    `json:"cause"`
	CreatedAt time.Time `json:"createdAt"`
	Clearable bool      `json:"clearable"`
}

type listResponse struct {
	Suppressions []suppressionDTO `json:"suppressions"`
}

// ListHandler answers GET /api/practices/{practiceId}/email-suppressions.
//
// This is the surface #744 chose over a per-record affordance on each
// mail kind's own screen. Eleven kinds send on the shared domain and
// only the Client portal invite showed a bounce anywhere; an
// address-keyed list matches how a suppression is actually keyed, and
// covers the ten kinds whose failures were invisible.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _ := staffauth.Tx(r.Context())
		practiceID, _ := staffauth.PracticeID(r.Context())

		items, err := List(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := listResponse{Suppressions: make([]suppressionDTO, 0, len(items))}
		for _, it := range items {
			resp.Suppressions = append(resp.Suppressions, suppressionDTO{
				Address:   it.Address,
				Cause:     it.Cause,
				CreatedAt: it.CreatedAt,
				Clearable: it.Cause == CauseBounce,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("mailsuppress.ListHandler: encode response: %v", err)
		}
	})
}

type clearRequest struct {
	Address string `json:"address"`
}

// ClearHandler answers POST
// /api/practices/{practiceId}/email-suppressions/clear.
//
// The address travels in the body rather than a path segment: a local
// part may legitimately contain '+' and '/', and a path segment is the
// one place those need escaping that callers reliably forget.
//
// Owner or Admin, per staffauth.RequireOwnerOrAdmin's own division --
// Owner-only guards who is at the Practice at all, and this is running
// the work: an address that bounced once cannot receive the invite,
// the Contract, or the payment notice until somebody lifts it.
func ClearHandler(clearer BounceClearer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		var req clearRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := Normalize(req.Address)
		if address == "" {
			apierr.WriteError(w, "an address is required", http.StatusBadRequest)
			return
		}

		// email_suppressions is platform-level with no RLS (migration
		// 00068), so nothing below this line knows one Practice from
		// another. This check is the boundary.
		attached, err := AttachedToPractice(r.Context(), tx, practiceID, address)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !attached {
			// Deliberately the same answer as "no suppression on that
			// address": a Practice must not learn from this endpoint
			// that another Practice's Client complained.
			apierr.WriteError(w, "no suppressed address of this Practice matches", http.StatusNotFound)
			return
		}

		switch err := Clear(r.Context(), tx, clearer, address, staffID); {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, ErrNotSuppressed):
			apierr.WriteError(w, "no suppressed address of this Practice matches", http.StatusNotFound)
		case errors.Is(err, ErrNotClearable):
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				"this address reported the email as spam, so it stays blocked", nil)
		default:
			// Mailgun refused the delete. Nothing local changed, which
			// is the point of doing the vendor call first.
			log.Printf("mailsuppress.ClearHandler: %v", err)
			apierr.WriteError(w, "could not reach the email provider; nothing was changed", http.StatusBadGateway)
		}
	})
}
