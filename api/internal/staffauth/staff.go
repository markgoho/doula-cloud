package staffauth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StaffSummary is one row of a Practice's roster: who they are, and what
// their Membership holds there.
type StaffSummary struct {
	StaffID        string   `json:"staffId"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
}

// InvitationSummary is one pending Invitation: an address that has been
// asked to join and has not answered yet. It is deliberately a different
// shape from StaffSummary rather than a member row with a flag -- a
// pending Invitation has no person behind it, so it has no name, no
// staff id, and nothing a Membership action can be taken against beyond
// revoking it (#261).
type InvitationSummary struct {
	InvitationID string   `json:"invitationId"`
	Address      string   `json:"address"`
	Roles        []string `json:"roles"`
	// EmploymentType is what the Membership will carry on acceptance.
	EmploymentType string `json:"employmentType"`
	ExpiresAt      string `json:"expiresAt"`
	// DeliveryFailed is true once the invitation email has exhausted its
	// retries and been dead-lettered (#339's passive Staff-visible flag).
	// It is not an error state of the Invitation itself -- the link still
	// works for anyone holding it -- only a statement that this Practice
	// should not assume the mail arrived.
	DeliveryFailed bool `json:"deliveryFailed"`
}

// Roster is the Staff screen's whole read: the people who are here, and
// the people who have been asked. Two groups, not one list with a status
// column, because #261 found the single-list shape unable to tell a
// pending invitation apart from a member holding no roles.
type Roster struct {
	Members     []StaffSummary      `json:"members"`
	Invitations []InvitationSummary `json:"invitations"`
}

// ListStaffHandler serves the Staff screen. Owner and Admin only
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

		members, err := listMembers(r, tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		invitations, err := listPendingInvitations(r, tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(Roster{Members: members, Invitations: invitations}); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func listMembers(r *http.Request, tx *sql.Tx, practiceID string) ([]StaffSummary, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT s.id, s.name, s.email, array_to_string(pm.roles, ','), pm.employment_type::text
		 FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1
		 ORDER BY s.name`,
		practiceID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("staffauth: query members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []StaffSummary{}
	for rows.Next() {
		var s StaffSummary
		var roles string
		if err := rows.Scan(&s.StaffID, &s.Name, &s.Email, &roles, &s.EmploymentType); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan member: %w", err)
		}
		s.Roles = splitRoles(roles)
		list = append(list, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("staffauth: iterate members: %w", err)
	}
	return list, nil
}

// listPendingInvitations reads the pending group. It checks expires_at
// rather than trusting the status column, for the reason
// staffinvite.Worker.ProcessPending gives: nothing sweeps expiries, so
// 'pending' outlives the window until someone tries to accept.
//
// The dead-letter flag is #339's passive Staff-visible signal, whose only
// read surface is this screen. It is a scalar subquery rather than a join
// because an Invitation can accumulate several staff_invite_outbox rows:
// staffinvite.Queue upserts against a partial index over pending rows
// only, so a re-invite after a send inserts a second one, and a join
// would print that Invitation twice. Reading only the newest attempt also
// gives the flag the meaning it should have -- "the last send gave up",
// which a re-invite clears -- rather than a permanent mark.
func listPendingInvitations(r *http.Request, tx *sql.Tx, practiceID string) ([]InvitationSummary, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT pi.id, pi.address, array_to_string(pi.roles, ','), pi.employment_type::text,
		        pi.expires_at,
		        COALESCE((SELECT o.status = 'dead_lettered'
		                  FROM staff_invite_outbox o
		                  WHERE o.invitation_id = pi.id
		                  ORDER BY o.created_at DESC
		                  LIMIT 1), false)
		 FROM practice_invitations pi
		 WHERE pi.practice_id = $1 AND pi.status = 'pending' AND pi.expires_at > now()
		 ORDER BY pi.created_at DESC`,
		practiceID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("staffauth: query invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []InvitationSummary{}
	for rows.Next() {
		var inv InvitationSummary
		var roles string
		var expiresAt time.Time
		if err := rows.Scan(&inv.InvitationID, &inv.Address, &roles, &inv.EmploymentType, &expiresAt, &inv.DeliveryFailed); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan invitation: %w", err)
		}
		inv.Roles = splitRoles(roles)
		inv.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		list = append(list, inv)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("staffauth: iterate invitations: %w", err)
	}
	return list, nil
}

// splitRoles turns array_to_string's output into a JSON array, mapping
// the empty string to an empty slice rather than to [""].
func splitRoles(csv string) []string {
	if csv == "" {
		return []string{}
	}
	return strings.Split(csv, ",")
}
