package staffauth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// StaffSummary is one row of a Practice's roster: who they are and what
// roles their membership holds there.
type StaffSummary struct {
	StaffID string   `json:"staffId"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
}

// ListStaffHandler lists the Staff members holding a membership at the
// current Practice, for the roster where role assignment and ending a
// person's sessions everywhere (#154) are reached. Owner and Admin only
// (ADR-0008's read table) -- a Doula has no reason to see the full
// roster; enforced by the "owner","admin" role declaration on this
// route's GatedRouter mount in main.go, not inside this handler. Must be
// mounted behind staffauth.Middleware.
func ListStaffHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		// coverage:ignore reason: Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		rows, err := tx.QueryContext(r.Context(),
			`SELECT s.id, s.name, s.email, array_to_string(pm.roles, ',')
			 FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1
			 ORDER BY s.name`,
			practiceID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = rows.Close() }()

		list := []StaffSummary{}
		for rows.Next() {
			var s StaffSummary
			var roles string
			if err := rows.Scan(&s.StaffID, &s.Name, &s.Email, &roles); err != nil {
				// coverage:ignore reason: row scan failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if roles != "" {
				s.Roles = strings.Split(roles, ",")
			} else {
				s.Roles = []string{}
			}
			list = append(list, s)
		}
		if err := rows.Err(); err != nil {
			// coverage:ignore reason: row iteration failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(list); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}
