package client

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientkey"
	"doula-cloud/api/internal/staffauth"
)

// CreateRequest is the body of a Client save. Every field but GivenName
// is optional -- intake is often typed on a phone by someone on call, and
// a fake value is worse than an empty one (ADR-0017). Override, when
// true, is ADR-0017's "No, a different person": it lets the write proceed
// even though FindMatches found a hit. There is no "this is her" flag
// here -- taking that path means editing the existing Client instead of
// calling this endpoint at all.
type CreateRequest struct {
	Record
	Override bool `json:"override"`
}

// CreateResponse is a refused create's body: the matches that stopped the
// write, so the screen can print "this is her" / "no, a different
// person" -- the matches are what the prompt needs, per ADR-0017.
type CreateResponse struct {
	Matches []Match `json:"matches"`
}

// CreateHandler saves a free-standing Client: lookup-before-insert, no
// Engagement, no Credit spent. Owner, Admin, or employee Doula only -- a
// contractor originates no Client at a Practice she does not belong to
// (ADR-0017); the clients_insert RLS policy enforces the same rule
// independently, so this check exists only to hand back a distinguishable
// error instead of a bare policy-denied failure. The lookup runs
// FindCollisions -- ADR-0017's amendment's exact-key predicate, not
// substring recall -- since nothing about this record exists yet to
// substitute a name on; any hit is a possible-duplicate question, which
// intake's own duplicate screen already answers. Must be mounted behind
// staffauth.Middleware.
func CreateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if reader.IsAmbientContractor() {
			apierr.WriteError(w, "a contractor doula does not create clients at a practice she contracts for -- work reaches her as an offer", http.StatusForbidden)
			return
		}

		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !normalizeAndValidate(w, &req.Record) {
			return
		}

		if !req.Override {
			collisions, err := FindCollisions(r.Context(), tx, practiceID, req.GivenName, req.FamilyName, req.DateOfBirth, req.Email, req.Phone, "")
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if len(collisions) > 0 {
				matches := make([]Match, len(collisions))
				for i, c := range collisions {
					matches[i] = c.Match
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				// coverage:ignore reason: response encoding failure, not exercised by unit tests
				if err := json.NewEncoder(w).Encode(CreateResponse{Matches: matches}); err != nil {
					apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				}
				return
			}
		}

		req.ID = uuid.NewString()
		if err := insertClient(r.Context(), tx, practiceID, req.Record); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// Her key is made in the same transaction that makes her, so a
		// Client and the key sealing her history either both exist or
		// neither does -- ADR-0027. It has to precede recordEvent, which
		// seals the created diff under it.
		if err := clientkey.Ensure(r.Context(), tx, practiceID, req.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordEvent(r.Context(), tx, practiceID, req.ID, eventCreated, createdDiff(req.Record), staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(req.Record); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// normalizeAndValidate trims every text field in place and confirms
// GivenName (the only required fact -- ADR-0017) and, if set, that
// DateOfBirth parses as YYYY-MM-DD. Writes its own 400 and returns false
// on failure.
func normalizeAndValidate(w http.ResponseWriter, rec *Record) bool {
	rec.GivenName = strings.TrimSpace(rec.GivenName)
	rec.FamilyName = strings.TrimSpace(rec.FamilyName)
	rec.PreferredName = strings.TrimSpace(rec.PreferredName)
	rec.Email = strings.TrimSpace(rec.Email)
	rec.Phone = strings.TrimSpace(rec.Phone)
	rec.AddressLine1 = strings.TrimSpace(rec.AddressLine1)
	rec.AddressLine2 = strings.TrimSpace(rec.AddressLine2)
	rec.AddressLocality = strings.TrimSpace(rec.AddressLocality)
	rec.AddressRegion = strings.TrimSpace(rec.AddressRegion)
	rec.AddressPostalCode = strings.TrimSpace(rec.AddressPostalCode)
	rec.DateOfBirth = strings.TrimSpace(rec.DateOfBirth)
	if len(rec.FieldValues) == 0 {
		rec.FieldValues = json.RawMessage("{}")
	}

	if rec.GivenName == "" {
		apierr.WriteError(w, "givenName is required", http.StatusBadRequest)
		return false
	}
	if rec.DateOfBirth != "" {
		if _, err := time.Parse(time.DateOnly, rec.DateOfBirth); err != nil {
			apierr.WriteError(w, "dateOfBirth must be YYYY-MM-DD", http.StatusBadRequest)
			return false
		}
	}
	return true
}
