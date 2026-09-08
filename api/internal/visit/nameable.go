package visit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// The reasons a Staff member cannot be named on a Visit at one
// Engagement, as stable machine-readable tokens. The screen turns each
// into words of its own (app/src/lib/staff.ts): no English crosses this
// boundary, so the wording can change without the contract changing
// (docs/api-design.md section 2).
const (
	// ReasonNotAtPractice: she holds no Membership at this Practice at all.
	ReasonNotAtPractice = "not_at_practice"
	// ReasonNotADoula: she is a Staff member here, but her Membership does
	// not carry the Doula role.
	ReasonNotADoula = "not_a_doula"
	// ReasonContractorWithoutAcceptedOffer: she is a contractor with no
	// open, granted attachment to this Engagement. CONTEXT.md's Attachment
	// entry: a contractor is put on a birth by her own acceptance of an
	// Offer and by nothing else, so sending her one is what changes this.
	ReasonContractorWithoutAcceptedOffer = "contractor_without_accepted_offer"
)

// grantedAttachmentPredicate is "she agreed to be on this birth", written
// once: an engagement_attachments row that is granted (never accrued --
// #228: a record of work, never a key) and still open. Both the write's
// per-person check (hasGrantedAttachment) and the read's set query below
// concatenate this const, so the two cannot drift apart. A const, not a
// fmt.Sprintf, so every query built from it stays a constant expression
// and gosec's G201/G202 have nothing to object to.
const grantedAttachmentPredicate = `origin = 'granted' AND ended_at IS NULL`

// decideNameable is the whole rule for whether a *named* Staff member may
// be put on a Visit at one Engagement, in one place: she is a Member here,
// her Membership carries the Doula role, and -- if she is a contractor --
// she already holds the attachment her own acceptance of an Offer opened.
// The empty string means she may be named.
//
// This is the single copy. requireEligibleAssignee (roles.go) calls it on
// the write and turns each reason into the refusal it has always written;
// AssigneesHandler below calls it on the read and hands each reason
// to the screen. The two gather the same facts differently -- the write
// asks about one person it was given, the read asks about a whole roster
// in two set queries -- but neither owns a second copy of the decision, so
// a picker cannot offer a name the write will refuse.
//
// Deliberately not given the caller's own identity: naming yourself is a
// different act with a different rule (see resolveAssignee in roles.go, and
// listVisitAssignees below).
func decideNameable(hasMembership, isDoula bool, employmentType string, attached bool) string {
	switch {
	case !hasMembership:
		return ReasonNotAtPractice
	case !isDoula:
		return ReasonNotADoula
	case employmentType != employeeType && !attached:
		return ReasonContractorWithoutAcceptedOffer
	}
	return ""
}

// refusalMessage is the sentence the write answers each reason with,
// unchanged from what it wrote before decideNameable existed -- the
// refusals are the contract, and this ticket moved none of them.
func refusalMessage(reason string) string {
	switch reason {
	case ReasonNotAtPractice:
		return "staff member not found at this practice"
	case ReasonNotADoula:
		return "staff member does not hold the Doula role at this practice"
	default:
		return "that contractor has not accepted an offer on this engagement"
	}
}

// Assignee is one row of the Visit pickers on the Engagement page:
// who she is, what she is called, what she is to the business, and
// whether she may be named on a Visit at *this* Engagement now.
//
// Nobody is filtered out. A contractor who has not accepted an Offer is
// precisely the person an Admin is trying to get onto the birth, and
// dropping her name would read as "she is not on the roster" -- so she
// comes back named, marked, and with the reason the screen turns into the
// act that changes it (#911).
type Assignee struct {
	StaffID        string `json:"staffId"`
	Name           string `json:"name"`
	EmploymentType string `json:"employmentType"`
	Nameable       bool   `json:"nameable"`
	Reason         string `json:"reason,omitempty"`
}

// AssigneesResponse is a bounded list, so it carries no cursor: a
// Practice's Doula roster is the same bounded set the Staff roster read
// returns in full (docs/api-design.md section 4 asks for a cursor on the
// collections that grow, and this one does not).
type AssigneesResponse struct {
	Items []Assignee `json:"items"`
}

// AssigneesHandler answers "who may be named on a Visit at this
// Engagement" -- the question the two Visit pickers could not ask before
// (#911). The pickers were drawn off the Practice-wide roster, which is
// Practice-scoped and so cannot know anything about one Engagement; the
// write applies a second, Engagement-scoped rule, so an Owner or Admin
// could pick a listed name and be refused for picking it.
//
// Owner/Admin, matching the audience of the roster read the pickers used
// before it (ADR-0006, ADR-0008): naming a colleague is scheduling, and a
// plain Doula has no roster to pick one from. Must be mounted behind
// staffauth.Middleware.
//
// A read surface only -- nothing here changes state, so there is nothing
// for the audit trail to record. It is drawing only, too: the BFF refuses
// the same write whatever this answered (requireEligibleAssignee), which
// is why both halves compute the answer from decideNameable rather than
// from two independent queries.
func AssigneesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		callerStaffID, hasStaffID := staffauth.StaffID(r.Context())
		if !hasStaffID {
			// coverage:ignore reason: staffauth.Middleware always places a staff id on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		list, err := listVisitAssignees(r.Context(), tx, practiceID, engagementID, callerStaffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, AssigneesResponse{Items: list})
	})
}

// doulaRosterQuery reads the Practice's Doulas, by name. Ordered so the
// picker's own order is decided here rather than by whatever order the
// heap gave back; s.id breaks the tie, because two Doulas at one agency
// can share a name.
const doulaRosterQuery = `SELECT s.id, s.name, m.employment_type::text
	  FROM practice_memberships m
	  JOIN staff s ON s.id = m.staff_id
	 WHERE m.practice_id = $1 AND $2 = ANY(m.roles)
	 ORDER BY s.name, s.id`

// grantedAttachmentsQuery reads, in one go, every Staff member holding the
// attachment this Engagement's contractors need. Two set queries rather
// than a per-person check inside the loop: a fourteen-doula agency would
// otherwise cost fourteen extra round trips to answer one picker, and the
// rule that reads the result is still decideNameable's, not this query's
// -- the SQL half it shares with the write is grantedAttachmentPredicate.
const grantedAttachmentsQuery = `SELECT staff_id FROM engagement_attachments
	 WHERE engagement_id = $1 AND ` + grantedAttachmentPredicate

// listVisitAssignees builds the answer: every Doula at the Practice, each
// marked with whether she may be named on this Engagement.
//
// The caller's own row is answered by the rule that actually applies to
// her, which is not the named-colleague rule. resolveAssignee (roles.go)
// takes the self path whenever the requested id equals the caller's, and
// requireEligibleAssignee never runs on it -- so a contractor Doula who
// owns or administers the Practice is accepted by the write when she
// names herself, and this read says so. Nameable-as-self is "she holds the
// Doula role", which the roster query above has already established for
// every row it returned. One rule for all rows would recreate the
// read/write disagreement this endpoint exists to close, pointing the
// other way.
func listVisitAssignees(
	ctx context.Context,
	tx *sql.Tx,
	practiceID, engagementID, callerStaffID string,
) ([]Assignee, error) {
	attached, err := grantedAttachmentHolders(ctx, tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, doulaRosterQuery, practiceID, doulaRole)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: list doula roster: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []Assignee{}
	for rows.Next() {
		var row Assignee
		if err := rows.Scan(&row.StaffID, &row.Name, &row.EmploymentType); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan doula roster row: %w", err)
		}
		if row.StaffID == callerStaffID {
			row.Nameable = true
		} else {
			row.Reason = decideNameable(true, true, row.EmploymentType, attached[row.StaffID])
			row.Nameable = row.Reason == ""
		}
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate doula roster rows: %w", err)
	}
	return list, nil
}

// grantedAttachmentHolders is hasGrantedAttachment asked of everybody at
// once: the set of staff ids holding an open, granted attachment to this
// Engagement.
func grantedAttachmentHolders(ctx context.Context, tx *sql.Tx, engagementID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, grantedAttachmentsQuery, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: list granted attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	held := map[string]bool{}
	for rows.Next() {
		var staffID string
		if err := rows.Scan(&staffID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan granted attachment row: %w", err)
		}
		held[staffID] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate granted attachment rows: %w", err)
	}
	return held, nil
}
