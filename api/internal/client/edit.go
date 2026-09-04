package client

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
)

// EditRequest is the body of a Client edit: a full replacement of Record,
// the same "object is the whole state" convention contracts.PutContract
// uses for Values. Override is ADR-0017's single deliberate override on
// this path -- "No, a different person" -- for when the edited values
// would otherwise match a different Client. There is deliberately no
// "this is her" option here: merging two records is out of scope.
type EditRequest struct {
	Record
	Override bool `json:"override"`
}

// EditHandler saves changes to an existing Client. Whoever may read a
// Client may edit her -- ADR-0017's "edit follows read" -- so access is
// Reader.CanAccessClient, the contractor carve-out included. The name-rule
// match query re-runs against the edited values (excluding her own row);
// a hit refuses the save unless Override is set. Changing the email
// revokes any pending portal invite in the same transaction. Must be
// mounted behind staffauth.Middleware.
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
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}

		old, err := fetchRecord(r.Context(), tx, practiceID, clientID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// An erased Client is not editable (ADR-0027). Her key is gone,
		// so recordEvent could not seal the edit's diff anyway -- but the
		// refusal is here, at the top and in words, rather than being
		// left to surface as a sealing failure further down.
		erased, err := isErased(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if erased {
			http.Error(w, "this client's data has been erased and cannot be edited", http.StatusConflict)
			return
		}

		var req EditRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !normalizeAndValidate(w, &req.Record) {
			return
		}
		req.ID = clientID

		if !req.Override {
			matches, err := FindMatches(r.Context(), tx, practiceID, req.GivenName, req.FamilyName, req.DateOfBirth, req.Email, req.Phone, clientID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if len(matches) > 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				// coverage:ignore reason: response encoding failure, not exercised by unit tests
				if err := json.NewEncoder(w).Encode(CreateResponse{Matches: matches}); err != nil {
					http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				}
				return
			}
		}

		if err := updateClient(r.Context(), tx, clientID, req.Record); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordEvent(r.Context(), tx, practiceID, clientID, eventUpdated, diffRecords(old, req.Record), staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if old.Email != req.Email {
			if err := portalinvite.RevokePending(r.Context(), tx, clientID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(req.Record); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
