package client

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// SearchRequest is what the search that fronts intake sends: the same
// four keys FindMatches always uses, any of them blank. A body rather
// than a query string (#1829): a query string is part of the request
// URL, and the Firebase Hosting rewrite and Cloud Run's request log both
// record the full URL, so a Client's name, date of birth, email and phone
// would sit in logs nothing else treats as holding them.
type SearchRequest struct {
	Name        string `json:"name"`
	DateOfBirth string `json:"dateOfBirth"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

// SearchResponse is the body of a Client search: every candidate
// FindMatches turned up, name and history unrestricted inside the
// Practice (ADR-0017).
type SearchResponse struct {
	Matches []Match `json:"matches"`
}

// SearchHandler is the search that fronts intake -- Clients -> Add a
// Client -> search -- the only door to it (ADR-0017). Reads a
// SearchRequest body; name matches against both given/family/preferred
// (see FindMatches). It writes nothing: it is a POST only so that what
// was typed stays out of the URL (#1829). Refused to a contractor Doula,
// the same as CreateHandler: she never originates a Client, so this
// search has no destination for her to reach. Must be mounted behind
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if reader.IsAmbientContractor() {
			apierr.WriteError(w, "a contractor doula does not search for clients at a practice she contracts for -- work reaches her as an offer", http.StatusForbidden)
			return
		}

		var req SearchRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		matches, err := FindMatches(r.Context(), tx, practiceID, req.Name, req.Name, req.DateOfBirth, req.Email, req.Phone, "")
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if matches == nil {
			matches = []Match{}
		}

		apierr.WriteJSON(w, http.StatusOK, SearchResponse{Matches: matches})
	})
}
