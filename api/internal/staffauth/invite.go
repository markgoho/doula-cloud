package staffauth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// InviteRequest is the body of an invite request: who the Owner is
// inviting to their Practice.
type InviteRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// InviteResponse identifies the pending Staff row invite created and
// hands back the one-time token the invitee needs to accept -- there is
// no email-sending infrastructure yet (see 00004_staff_invitation.sql),
// so the Owner is expected to pass this along outside the app.
type InviteResponse struct {
	StaffID     string `json:"staffId"`
	InviteToken string `json:"inviteToken"`
}

// InviteHandler lets a Practice Owner invite another person to their
// Practice: it creates a pending Staff row (no identity_uid yet) and a
// practice_memberships row with zero roles -- the invite is not usable
// for anything until an Owner calls AssignRolesHandler. Must be mounted
// behind staffauth.Middleware.
func InviteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireOwner(w, r)
		if !ok {
			return
		}

		var req InviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		req.Name = strings.TrimSpace(req.Name)
		if req.Email == "" || req.Name == "" {
			http.Error(w, "email and name are required", http.StatusBadRequest)
			return
		}

		// Generated in Go, not via `RETURNING id`: the pending row (no
		// identity_uid yet) doesn't match any SELECT policy on staff, and
		// Postgres applies SELECT policies to RETURNING rows too.
		newStaffID := uuid.NewString()
		inviteToken := uuid.NewString()

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO staff (id, name, email, invite_token) VALUES ($1, $2, $3, $4)`,
			newStaffID, req.Name, req.Email, inviteToken,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{}')`,
			practiceID, newStaffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(InviteResponse{StaffID: newStaffID, InviteToken: inviteToken}); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}
