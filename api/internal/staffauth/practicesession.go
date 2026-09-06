package staffauth

import (
	"encoding/json"
	"log"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// PracticeSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// Middleware, records it as the Staff member's last-used Practice for
// their next login.
//
// IsContractor carries ADR-0008's employment-type axis alongside Roles.
// It exists because #501's contractor Add-a-Client door needs to branch
// on employment type before ever calling client.SearchHandler -- the same
// UX-only mirror of a BFF role gate this endpoint's Roles field already
// is for the Owner/Admin screens that read it.
type PracticeSessionResponse struct {
	PracticeID   string   `json:"practiceId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
	IsContractor bool     `json:"isContractor"`
}

// PracticeSessionHandler answers GET .../session: which Practice, which
// roles, which employment type. Moved from routes_practice.go by #836 --
// the handler already used nothing but this package's own exports
// (Tx, PracticeID, ReaderFrom), so it belongs here rather than in main.
func PracticeSessionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _ := Tx(r.Context())
		practiceID, _ := PracticeID(r.Context())

		var name string
		if err := tx.QueryRowContext(r.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		reader, has := ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := PracticeSessionResponse{
			PracticeID:   practiceID,
			PracticeName: name,
			Roles:        reader.Roles(),
			IsContractor: reader.IsContractor(),
		}
		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("PracticeSessionHandler: encode response: %v", err)
		}
	})
}
