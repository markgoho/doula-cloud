package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

// invitationPageSize is the fixed number of pending Invitations returned
// per page, matching message.pageSize's reasoning.
const invitationPageSize = 30

// maxMembers caps the roster query rather than paginating it: #446's
// ticket asks that the roster's two collections each be handled
// deliberately, and a Practice's roster is a bounded population (the
// pilot is a 14-doula agency) while its Invitation history is not. The
// cap exists only so AC2's "every query carries a LIMIT" holds even for
// a query that never needs to page -- it is headroom, not an expected
// ceiling.
const maxMembers = 500

// StaffSummary is one row of a Practice's roster: who they are, and what
// their Membership holds there.
type StaffSummary struct {
	StaffID        string   `json:"staffId"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
	// WorkState is the US state this person works from (#415), and
	// WorkStateReportedAt is when she last asserted it. Both are on the
	// roster because only she may write the value, so an Owner reading
	// "New York -- self-reported 28 Aug 2026" has the whole answer to
	// "how did this get set?" -- including for a contractor who recorded
	// it at another Practice before this one existed. The date is also
	// the only staleness signal there is: nothing prompts a re-assertion.
	WorkState           string    `json:"workState"`
	WorkStateReportedAt time.Time `json:"workStateReportedAt"`
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
	// Expired is true once expires_at has passed. The Invitation is still
	// listed: nothing sweeps expiries, so it keeps its slot in
	// practice_invitations_one_pending until it is revoked or re-sent, and
	// an Owner who cannot see it cannot act on it.
	Expired bool `json:"expired"`
	// DeliveryFailed is true once the invitation email has exhausted its
	// retries and been dead-lettered (#339's passive Staff-visible flag).
	// It is not an error state of the Invitation itself -- the link still
	// works for anyone holding it -- only a statement that this Practice
	// should not assume the mail arrived.
	DeliveryFailed bool `json:"deliveryFailed"`

	createdAt time.Time
}

// InvitationPage is the cursor-pagination envelope from
// docs/api-design.md section 4, nested under Roster.Invitations -- see
// Roster's own doc comment for why Members stays a bare slice.
type InvitationPage struct {
	Items      []InvitationSummary `json:"items"`
	NextCursor *string             `json:"nextCursor,omitempty"`
	HasMore    bool                `json:"hasMore"`
}

// Roster is the Staff screen's whole read: the people who are here, and
// the people who have been asked. Two groups, not one list with a status
// column, because #261 found the single-list shape unable to tell a
// pending invitation apart from a member holding no roles.
//
// Members stays a bare, capped slice and Invitations cursor-paginates --
// #446's ticket asks that this split be deliberate rather than
// mechanical: a roster is bounded (the pilot is a 14-doula agency) but a
// Practice's Invitation history is not, since nothing sweeps or deletes
// a lapsed one (see listPendingInvitations).
type Roster struct {
	Members     []StaffSummary `json:"members"`
	Invitations InvitationPage `json:"invitations"`
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

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		members, err := listMembers(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		invitations, err := listPendingInvitations(r.Context(), tx, practiceID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(invitations) > invitationPageSize
		if hasMore {
			invitations = invitations[:invitationPageSize]
		}
		invitationPage := InvitationPage{Items: invitations, HasMore: hasMore}
		if hasMore {
			last := invitations[len(invitations)-1]
			next := pagecursor.Encode(last.createdAt, last.InvitationID)
			invitationPage.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(Roster{Members: members, Invitations: invitationPage}); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func listMembers(ctx context.Context, tx *sql.Tx, practiceID string) ([]StaffSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.id, s.name, s.email, array_to_string(pm.roles, ','), pm.employment_type::text,
		        s.work_state, s.work_state_reported_at
		 FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1
		 ORDER BY s.name
		 LIMIT $2`,
		practiceID, maxMembers,
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
		if err := rows.Scan(&s.StaffID, &s.Name, &s.Email, &roles, &s.EmploymentType, &s.WorkState, &s.WorkStateReportedAt); err != nil {
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

// listPendingInvitations reads the pending group -- every row still
// holding 'pending', lapsed or not. "Pending" means exactly what
// practice_invitations_one_pending (00039) means by it, because nothing
// sweeps expiries: a lapsed Invitation still occupies that address's slot
// until it is revoked or re-sent, so hiding it would leave the Owner
// unable to act on the thing blocking her. Whether it has lapsed rides
// along as Expired instead.
//
// The dead-letter flag is #339's passive Staff-visible signal, whose only
// read surface is this screen. It is a scalar subquery rather than a join
// because an Invitation can accumulate several staff_invite_outbox rows:
// staffinvite.Queue upserts against a partial index over pending rows
// only, so a re-invite after a send inserts a second one, and a join
// would print that Invitation twice. Reading only the newest attempt also
// gives the flag the meaning it should have -- "the last send gave up",
// which a re-invite clears -- rather than a permanent mark.
func listPendingInvitations(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]InvitationSummary, error) {
	query := `SELECT pi.id, pi.address, array_to_string(pi.roles, ','), pi.employment_type::text,
		        pi.expires_at, pi.expires_at <= now(),
		        COALESCE((SELECT o.status = 'dead_lettered'
		                  FROM staff_invite_outbox o
		                  WHERE o.invitation_id = pi.id
		                  ORDER BY o.created_at DESC
		                  LIMIT 1), false),
		        pi.created_at
		 FROM practice_invitations pi
		 WHERE pi.practice_id = $1 AND pi.status = 'pending'`
	args := []any{practiceID}
	if after != nil {
		query += ` AND (pi.created_at, pi.id) < ($2, $3) ORDER BY pi.created_at DESC, pi.id DESC LIMIT $4`
		args = append(args, after.At, after.ID, invitationPageSize+1)
	} else {
		query += ` ORDER BY pi.created_at DESC, pi.id DESC LIMIT $2`
		args = append(args, invitationPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
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
		if err := rows.Scan(&inv.InvitationID, &inv.Address, &roles, &inv.EmploymentType, &expiresAt, &inv.Expired, &inv.DeliveryFailed, &inv.createdAt); err != nil {
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
