package client

import (
	"encoding/json"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// SearchResponse is the body of a Client search: every candidate
// FindMatches turned up, name and history unrestricted inside the
// Practice (ADR-0017).
type SearchResponse struct {
	Matches []Match `json:"matches"`
}

// SearchHandler is the search that fronts intake -- Clients -> Add a
// Client -> search -- the only door to it (ADR-0017). Reads
// ?name=&dateOfBirth=&email=&phone=, matching the same four keys
// FindMatches always uses; name matches against both given/family/
// preferred (see FindMatches). Refused to a contractor Doula, the same
// as CreateHandler: she never originates a Client, so this search has no
// destination for her to reach. Must be mounted behind
// staffauth.Middleware.
func SearchHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if reader.IsAmbientContractor() {
			apierr.WriteError(w, "a contractor doula does not search for clients at a practice she contracts for -- work reaches her as an offer", http.StatusForbidden)
			return
		}

		q := r.URL.Query()
		name := q.Get("name")
		matches, err := FindMatches(r.Context(), tx, practiceID, name, name, q.Get("dateOfBirth"), q.Get("email"), q.Get("phone"), "")
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if matches == nil {
			matches = []Match{}
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(SearchResponse{Matches: matches}); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
