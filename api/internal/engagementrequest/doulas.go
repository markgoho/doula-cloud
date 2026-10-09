package engagementrequest

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// RequestDoula is one answer the Start work form offers to "Who is the
// Doula?": a person the caller may name on a Request now.
type RequestDoula struct {
	StaffID string `json:"staffId"`
	Name    string `json:"name"`
}

// DoulasResponse is the form's whole list, and the one fact that decides
// whether an answer is selected when the form opens.
//
// A bounded list, so it carries no cursor: a Practice's Doula roster is
// the same bounded set the Staff roster read returns in full
// (docs/api-design.md section 4 asks for a cursor on the collections that
// grow, and this one does not).
//
// CallerIsOnlyDoula is true where the caller is in Items and no other
// Member of the Practice holds the Doula role. ADR-0017's amendment on
// #1515: "where the asker is the only Doula at the Practice, she is
// selected already". A contractor Doula who holds neither Owner nor Admin
// counts as another Doula, although she is never in Items: with her at
// the Practice, "the Doula is me" is no longer the only answer that work
// can have, and the departure recorded in docs/design/govuk-alignment.md
// stops "the moment a second Doula joins the Practice". The fact is
// computed here and not on the screen, because a plain Doula's Items
// holds herself alone at a Practice of any size.
//
// It is the one fact about the roster a plain Doula reads here: whether
// any other Doula is at her Practice, and nothing about who. ADR-0008
// gives her no read of the roster, and ADR-0017's amendment asks for
// this fact for every asker, so it is one bit and no more -- never a
// count, and never a name.
type DoulasResponse struct {
	Items             []RequestDoula `json:"items"`
	CallerIsOnlyDoula bool           `json:"callerIsOnlyDoula"`
}

// msgContractorOriginates is the 403 a contractor Doula reads on both
// halves of an ask, the write and the list that feeds its form.
const msgContractorOriginates = "a contractor doula does not request an engagement at a practice she contracts for -- work reaches her as an offer"

// DoulasHandler answers "who may this person name as the Doula on a new
// Request" for the Start work form (#1596).
//
// For an Owner or an Admin: each Doula at the Practice who can be
// attached without an Offer, which is each employee Doula and each Doula
// who holds Owner or Admin, herself included (ADR-0008's amendment on
// #1625). For any other Staff member: herself, where she is an employee
// Doula, and nobody else, because ADR-0008 gives a plain Doula no read of
// the roster. A contractor who holds neither Owner nor Admin is never
// listed, and a person whose Invitation is pending holds no Membership
// and so is not in the read at all. The caller is first, and the rest
// are in name order.
//
// Any Staff member may call it but a contractor Doula, who originates
// nothing (ADR-0017) and is refused here as RequestHandler refuses her.
// A read only: it changes no state, so the audit trail has nothing to
// record. It is drawing only, too: RequestHandler refuses the same names
// whatever this answered, and the two share attachment.Membership.WhyNotAttachable. Must be
// mounted behind staffauth.Middleware.
func DoulasHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		callerStaffID, _ := staffauth.StaffID(r.Context())
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if isContractorOriginator(reader) {
			apierr.WriteError(w, msgContractorOriginates, http.StatusForbidden)
			return
		}

		resp, err := listRequestDoulas(r.Context(), tx, practiceID, callerStaffID, mayApproveDirectly(reader))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// doulaRole is the practice_role the roster read below selects on.
const doulaRole = "doula"

// doulaRosterQuery reads every Member of the Practice who holds the Doula
// role, the caller first and the rest by name. s.id breaks the tie,
// because two Doulas at one agency can share a name. One query for the
// whole roster, whatever its size. It reads the three facts the
// attachment rule needs of a Member who holds the Doula role.
const doulaRosterQuery = `SELECT s.id, s.name, m.employment_type::text, ` + attachment.OwnerOrAdminSQL + `
	  FROM practice_memberships m
	  JOIN staff s ON s.id = m.staff_id
	 WHERE m.practice_id = $1 AND $2 = ANY(m.roles)
	 ORDER BY (s.id = $3) DESC, s.last_name, s.first_name, s.id`

// listRequestDoulas builds the answer. readsRoster is whether the caller
// may be shown a colleague: true for an Owner or an Admin.
func listRequestDoulas(ctx context.Context, tx *sql.Tx, practiceID, callerStaffID string, readsRoster bool) (DoulasResponse, error) {
	rows, err := tx.QueryContext(ctx, doulaRosterQuery, practiceID, doulaRole, callerStaffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DoulasResponse{}, fmt.Errorf("engagementrequest: list doula roster: %w", err)
	}
	defer func() { _ = rows.Close() }()

	resp := DoulasResponse{Items: []RequestDoula{}}
	doulas, callerListed := 0, false
	for rows.Next() {
		var row RequestDoula
		// Every row the query returns is a Member who holds the Doula
		// role, so the rule has her Employment type and her Owner-or-Admin
		// fact left to read.
		listed := attachment.Membership{Exists: true, IsDoula: true}
		if err := rows.Scan(&row.StaffID, &row.Name, &listed.EmploymentType, &listed.IsOwnerOrAdmin); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return DoulasResponse{}, fmt.Errorf("engagementrequest: scan doula roster row: %w", err)
		}
		doulas++
		isCaller := row.StaffID == callerStaffID
		if listed.WhyNotAttachable() != attachment.Attachable || (!isCaller && !readsRoster) {
			continue
		}
		callerListed = callerListed || isCaller
		resp.Items = append(resp.Items, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return DoulasResponse{}, fmt.Errorf("engagementrequest: iterate doula roster rows: %w", err)
	}
	resp.CallerIsOnlyDoula = callerListed && doulas == 1
	return resp, nil
}
