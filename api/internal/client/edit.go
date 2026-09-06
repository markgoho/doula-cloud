package client

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
)

// EditRequest is the body of a Client edit: a full replacement of Record,
// the same "object is the whole state" convention contracts.PutContract
// uses for Values. Override is ADR-0017's amendment: the single
// deliberate "No, a different person" act that answers either gate --
// gate one's substitution block, or gate two's possible-duplicate
// question -- and skips FindCollisions entirely on retry. Gate two's
// other answer, "This is her", is not a flag here: it is MergeHandler, a
// different endpoint, because it writes a different record (the match)
// and tombstones this one.
type EditRequest struct {
	Record
	Override bool `json:"override"`
}

// EditConflictResponse is a refused edit's body -- ADR-0017's amendment,
// one wire shape for both gates. Substitution true is gate one: a name
// column changed and the result is exactly another Client's given and
// family name; Matches is just the substituted-into record(s), and
// MergeOffered is always false -- Override is the only next step.
// Substitution false is gate two: a possible duplicate, asked rather
// than blocked; MergeOffered says whether "This is her" is available at
// all (ADR-0017's amendment: only while the record being edited holds no
// Engagement, no Engagement Request, no portal invitation and no portal
// account), and each match's WouldSurvive says which side a "This is
// her" answer on it would keep -- computed here, not by the frontend, so
// the changes it lists are the changes MergeHandler would actually make
// (direction never depends on which record is open).
type EditConflictResponse struct {
	Matches      []CollisionMatch `json:"matches"`
	Substitution bool             `json:"substitution"`
	MergeOffered bool             `json:"mergeOffered"`
}

// CollisionMatch is one Match FindCollisions turned up, plus whether it
// would survive a merge -- meaningless (and left false) outside gate
// two's MergeOffered case.
type CollisionMatch struct {
	Match
	WouldSurvive bool `json:"wouldSurvive"`
}

// writeConflict writes a 409 EditConflictResponse, the shared tail of
// both gates below.
func writeConflict(w http.ResponseWriter, body EditConflictResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(body); err != nil {
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
	}
}

// EditHandler saves changes to an existing Client. Whoever may read a
// Client may edit her -- ADR-0017's "edit follows read" -- so access is
// Reader.CanAccessClient, the contractor carve-out included. Unless
// Override is set, the collision predicate (FindCollisions) re-runs
// against the edited values, excluding her own row, and sorts a hit into
// one of ADR-0017's amendment's two gates: gate one blocks a name
// substitution, gate two asks about a possible duplicate and writes
// nothing until it is answered. Changing the email revokes any pending
// portal invite in the same transaction. Must be mounted behind
// staffauth.Middleware.
func EditHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}

		old, err := fetchRecord(r.Context(), tx, practiceID, clientID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// An erased Client is not editable (ADR-0027). Her key is gone,
		// so recordEvent could not seal the edit's diff anyway -- but the
		// refusal is here, at the top and in words, rather than being
		// left to surface as a sealing failure further down.
		erased, err := isErased(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if erased {
			apierr.WriteError(w, "this client's data has been erased and cannot be edited", http.StatusConflict)
			return
		}

		// A tombstoned row is not editable either (ADR-0017's amendment).
		// clients_update's own USING clause (00080) would silently match
		// zero rows on the UPDATE below rather than error -- this refusal
		// is what turns that into an honest response instead of a 200
		// that wrote nothing.
		mergedInto, err := readMergedInto(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if mergedInto != nil {
			apierr.WriteError(w, "this client record has been merged into another and cannot be edited", http.StatusConflict)
			return
		}

		var req EditRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !normalizeAndValidate(w, &req.Record) {
			return
		}
		req.ID = clientID

		if !req.Override {
			collisions, err := FindCollisions(r.Context(), tx, practiceID, req.GivenName, req.FamilyName, req.DateOfBirth, req.Email, req.Phone, clientID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}

			// "A name column changed in this edit" reads all three name
			// columns, including preferred -- gate one's own equality test
			// (FindCollisions' NameSubstitution) compares given and family
			// name only, never preferred, because two Clients can both be
			// called Bex (#727's resolution).
			nameChanged := !strings.EqualFold(strings.TrimSpace(old.GivenName), strings.TrimSpace(req.GivenName)) ||
				!strings.EqualFold(strings.TrimSpace(old.FamilyName), strings.TrimSpace(req.FamilyName)) ||
				!strings.EqualFold(strings.TrimSpace(old.PreferredName), strings.TrimSpace(req.PreferredName))
			if nameChanged {
				var substitutions []CollisionMatch
				for _, c := range collisions {
					if c.NameSubstitution {
						substitutions = append(substitutions, CollisionMatch{Match: c.Match})
					}
				}
				if len(substitutions) > 0 {
					writeConflict(w, EditConflictResponse{Matches: substitutions, Substitution: true})
					return
				}
			}

			if len(collisions) > 0 {
				sourceAttached, err := isAttachedRecord(r.Context(), tx, clientID)
				if err != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
					return
				}
				mergeOffered := !sourceAttached

				var sourceCreatedAt time.Time
				if mergeOffered {
					sourceCreatedAt, err = clientCreatedAt(r.Context(), tx, clientID)
					if err != nil {
						// coverage:ignore reason: DB query failure, not exercised by unit tests
						apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
						return
					}
				}

				matches := make([]CollisionMatch, len(collisions))
				for i, c := range collisions {
					match := CollisionMatch{Match: c.Match}
					if mergeOffered {
						otherAttached, err := isAttachedRecord(r.Context(), tx, c.ID)
						if err != nil {
							// coverage:ignore reason: DB query failure, not exercised by unit tests
							apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
							return
						}
						match.WouldSurvive = resolveMergeDirection(otherAttached, sourceCreatedAt, c.CreatedAt)
					}
					matches[i] = match
				}
				writeConflict(w, EditConflictResponse{Matches: matches, MergeOffered: mergeOffered})
				return
			}
		}

		if err := updateClient(r.Context(), tx, clientID, req.Record); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordEvent(r.Context(), tx, practiceID, clientID, eventUpdated, diffRecords(old, req.Record), staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if old.Email != req.Email {
			if err := portalinvite.RevokePending(r.Context(), tx, clientID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(req.Record); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
